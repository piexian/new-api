package model

import (
	"time"

	"gorm.io/gorm"
)

// CloseAccountForDeletion 账号删除前的资产清算（自助注销与管理员删除共用）：
// 钱包余额 + 有效套餐未消耗额度全部并入抵债预算，按「先本后息」强制还债
// （被动清算，豁免提前还款手续费与秒结清罚则）；抵不完的 funding 逐条核销
// （written_off 终态）；最后有效套餐全部取消、钱包余额清零。
// 台账 Source = account_closure。无贷款账户、无有效套餐且无余额的用户快速返回。
// 清算成功后调用方继续执行删除；返回错误时调用方应中止删除（不产生半删除状态）。
func CloseAccountForDeletion(userId int) error {
	now := time.Now()
	var credits []LenderCredit
	var walletDeducted int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}

		// 有效套餐（id 升序锁定）：未消耗额度并入抵债预算，清算结束统一取消
		var subs []UserSubscription
		if err := lockForUpdate(tx).Where("user_id = ? AND status = ?", userId, "active").
			Order("id ASC").Find(&subs).Error; err != nil {
			return err
		}
		var subBudget int64
		for i := range subs {
			if remaining := subs[i].AmountTotal - subs[i].AmountUsed; remaining > 0 {
				subBudget += remaining
			}
		}

		acc, err := getLoanAccountTx(tx, userId)
		if err != nil {
			return err
		}
		if acc == nil && len(subs) == 0 && user.Quota == 0 {
			return nil // 无贷无套餐无余额：快速返回
		}

		if acc != nil {
			fundings, err := loadUserFundingsTx(tx, userId)
			if err != nil {
				return err
			}
			// 先以 acc 全量结算一遍（platform funding 的利率/宽限需要 acc 输入），
			// 后续 distributeRepayment/writeoffFundingTx 内部结算均为幂等空操作
			for i := range fundings {
				before := fundings[i]
				settleFunding(&fundings[i], acc, now)
				if fundings[i].DebtQuota != before.DebtQuota || fundings[i].LastSettledDay != before.LastSettledDay {
					if err := tx.Save(&fundings[i]).Error; err != nil {
						return err
					}
				}
			}
			syncAccountFromFundings(acc, fundings)

			wallet := int64(user.Quota)
			if wallet < 0 {
				wallet = 0
			}
			budget := wallet + subBudget
			var totalDebt int64
			for i := range fundings {
				if fundings[i].DebtQuota > 0 {
					totalDebt += fundings[i].DebtQuota
				}
			}

			if budget > 0 && totalDebt > 0 {
				// 先本后息强制还债（分配额 = min(budget, Σdebt)），豁免手续费/秒结罚则
				info, allocs, _, err := distributeRepayment(tx, acc, fundings, budget, now, "account_closure", true)
				if err != nil {
					return err
				}
				if info != nil && info.Amount > 0 {
					credits, err = settleRepayAllocations(tx, userId, allocs, "account_closure", nil, 0)
					if err != nil {
						return err
					}
					// 出资扣减：先钱包后套餐（id 升序），套餐按出资额增加 AmountUsed
					rest := info.Amount
					walletDeducted = min(rest, wallet)
					rest -= walletDeducted
					for i := range subs {
						if rest <= 0 {
							break
						}
						if remaining := subs[i].AmountTotal - subs[i].AmountUsed; remaining > 0 {
							take := min(rest, remaining)
							subs[i].AmountUsed += take
							rest -= take
						}
					}
				}
			}

			// 账户投影落盘须在核销循环之前：writeoffFundingTx 内部会重新加载账户、
			// 销毁已核销债务的投影并自行落盘；若之后再 Save 旧 acc 会覆盖核销结果
			acc.UpdatedAt = now.Unix()
			if err := tx.Save(acc).Error; err != nil {
				return err
			}

			// 残余债务核销：结算/还款后仍 active/overdue 的 funding 逐条核销
			// （终态 written_off + offer 核减 + 信用台账，复用既有不变量）
			for i := range fundings {
				if fundings[i].Status != LoanFundingActive && fundings[i].Status != LoanFundingOverdue {
					continue
				}
				if err := writeoffFundingTx(tx, &fundings[i], now); err != nil {
					return err
				}
			}
		}

		// 套餐全部取消 + 钱包余额清零（无论是否有贷）
		for i := range subs {
			subs[i].Status = "cancelled"
			subs[i].EndTime = now.Unix()
			subs[i].UpdatedAt = now.Unix()
			if err := tx.Save(&subs[i]).Error; err != nil {
				return err
			}
		}
		return tx.Model(&User{}).Where("id = ?", userId).Update("quota", 0).Error
	})
	if err != nil {
		// 放贷人入账溢出等防御（同 RepayLoan）：事务已回滚，异步通知介入
		notifyLenderOverflowAsync(err)
		return err
	}
	// 提交后异步同步 Redis 余额缓存（镜像 RepayLoan 副作用）
	go func() {
		if walletDeducted > 0 {
			_ = cacheDecrUserQuota(userId, walletDeducted)
		}
		for _, c := range credits {
			_ = cacheIncrUserQuota(c.UserId, c.Amount)
		}
	}()
	return nil
}
