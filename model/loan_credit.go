package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// ===== Task 13: 信用分引擎 =====

// 信用分硬边界（spec §4.1 / P1-9）：罚分下限 -50、加分上限 100。
// 核销扣分路径（writeoffFundingTx）自带上限截断，此处只约束本文件的分支。
const (
	creditScoreFloor = -50
	creditScoreCeil  = 100
)

// scoreBorrowEventRepaidTx 在事务内对「借款事件」做信用分结算：事件的某条 funding 转
// repaid 时调用，仅当事件全部 funding 均已结清才评分（事件级加分/扣分至多一次）。
//
// 幂等性论证（为什么不会双评）：本函数只由 distributeRepayment 在 ③ 循环后调用，且只对
// 本次调用中转为 repaid 的 funding 所属事件（去重后）各调用一次。distributeRepayment 的
// 输入只含 active/overdue funding（loadUserFundingsTx），③ 中 debt 归零才会置 repaid——
// 故传入切片里出现 repaid 态的 funding 必然是本次调用转结清的。同一事件全部 funding 转
// repaid 后，该事件不再有任何 active/overdue 行，后续任何还款流程都不可能再载入它们，
// 也就不可能对同一事件发起第二次评分调用。因此「调用时事件全部 funding 已 repaid」
// ⇔「本次调用恰好把事件最后一条 funding 转结清」，且函数内 all-repaid 检查保证只有
// 这个时刻才评分——双评（同一事件 +5 两次）不可能发生。
//
// 规则（spec §10 / plan Task 13）：
//   - BorrowEventId == 0 的 legacy funding（迁移生成，无对应 borrow 台账行）直接跳过；
//   - borrow 台账行（Type="borrow"，id = borrowEventId）缺失 → 跳过（防御，不评分）；
//   - 持有天数 heldDays = loanDay(now) - loanDay(record.CreatedAt)；
//   - heldDays < CreditMinHoldDays → 快速还清扣 CreditFastRepayPenalty（反刷分），
//     下限 -50 钳制后直接结束（不再走按时判定）；
//   - 否则按事件内 max(due_day) 判定（含延长处置写回的新 due_day）：today > max(due_day)
//     → 逾期后还清，不加分不扣分；today <= max(due_day) 且事件本金（record.Amount）
//     换算 USD >= CreditMinBorrowUsd → 按时还清加 CreditRepayBonus，上限 100 钳制；
//     本金低于门槛（刷分墙）→ 不加分不扣分。
//
// 评分发生时（加分/扣分两个分支）各写一条 type=credit 台账行：Amount=实际生效的
// 加减分（钳制后 new-old），DebtAfter 复用为变动后信用分，Source=repay_bonus /
// fast_repay，RefId=借款事件 id。仅修改内存 acc（CreditScore），落盘由调用方统一
// tx.Save(acc)（与 maybeLiftBlacklistTx 同风格），台账行在本事务内直接写入。
func scoreBorrowEventRepaidTx(tx *gorm.DB, acc *TokenLoanAccount, borrowEventId int64, now time.Time) error {
	if borrowEventId <= 0 {
		return nil // legacy 迁移 funding：无对应借款事件，不评分
	}

	// 事件全部 funding（任意状态，含 written_off）：非全部 repaid 说明事件未完全结清，
	// 等最后一条转 repaid 时再由调用方触发评分。written_off 是终态，永远不等于 repaid，
	// 故存在核销行的事件永远不会被加分——违约扣分已由核销路径（Task 12）处理。
	var eventFundings []TokenLoanFunding
	if err := tx.Where("borrow_event_id = ?", borrowEventId).Find(&eventFundings).Error; err != nil {
		return err
	}
	if len(eventFundings) == 0 {
		return nil // 防御：事件无 funding 行（数据异常），不评分
	}
	repaidCnt := 0
	maxDueDay := 0
	for i := range eventFundings {
		f := &eventFundings[i]
		if f.Status == LoanFundingRepaid {
			repaidCnt++
		}
		if f.DueDay > maxDueDay {
			maxDueDay = f.DueDay
		}
	}
	if repaidCnt < len(eventFundings) {
		return nil // 事件未完全结清
	}

	// borrow 台账行缺失 → 跳过（防御：事件无本金/时间基准，不评分）
	var record TokenLoanRecord
	if err := tx.Where("id = ? AND type = ?", borrowEventId, "borrow").First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	loanSetting := operation_setting.GetLoanSetting()
	heldDays := loanDay(now) - loanDay(time.Unix(record.CreatedAt, 0))
	if heldDays < loanSetting.CreditMinHoldDays {
		// 快速还清（反刷分）：扣分后直接结束，不再按 due_day 判定
		before := acc.CreditScore
		acc.CreditScore -= loanSetting.CreditFastRepayPenalty
		if acc.CreditScore < creditScoreFloor {
			acc.CreditScore = creditScoreFloor
		}
		return writeCreditLedgerRowTx(tx, acc, before, "fast_repay", borrowEventId, now)
	}
	// 逾期后还清：不加分不扣分（基准 = 事件内 max(due_day)，含延长后的新 due_day）
	if loanDay(now) > maxDueDay {
		return nil
	}
	// 刷分墙：事件本金（borrow 行 Amount，quota）换算 USD 低于门槛 → 不计分
	if float64(record.Amount)/common.QuotaPerUnit < loanSetting.CreditMinBorrowUsd {
		return nil
	}
	before := acc.CreditScore
	acc.CreditScore += loanSetting.CreditRepayBonus
	if acc.CreditScore > creditScoreCeil {
		acc.CreditScore = creditScoreCeil
	}
	return writeCreditLedgerRowTx(tx, acc, before, "repay_bonus", borrowEventId, now)
}

// writeCreditLedgerRowTx 写一条信用分变动台账行（type=credit）：
//   - Amount = 实际生效的加减分 = 钳制后的 new - before（钳制时可能小于名义值，如
//     99 + 5 → +1、-45 - 20 → -5），保证台账与账户最终分一致；
//   - DebtAfter 复用为变动后信用分（字段语义见 model/loan.go TokenLoanRecord）；
//   - Source 为变动原因（repay_bonus / fast_repay / writeoff），RefId 关联借款事件
//     （核销无事件，传 0）；InterestPart/PrincipalPart/FeePart/FundingId/LenderId = 0。
//
// delta == 0（如对应参数配置为 0）时无实际变动，不写行，避免噪音记录。
func writeCreditLedgerRowTx(tx *gorm.DB, acc *TokenLoanAccount, before int, source string, refId int64, now time.Time) error {
	delta := acc.CreditScore - before
	if delta == 0 {
		return nil
	}
	record := &TokenLoanRecord{
		UserId:    acc.UserId,
		Type:      "credit",
		Amount:    int64(delta),
		DebtAfter: int64(acc.CreditScore),
		Source:    source,
		RefId:     refId,
		CreatedAt: now.Unix(),
	}
	return tx.Create(record).Error
}

// GetCreditScore 查询用户当前信用分：账户不存在时返回信用分初始值（CreditInitial，不建行）。
func GetCreditScore(userId int) (int, error) {
	acc, err := GetLoanAccountReadOnly(userId)
	if err != nil {
		return 0, err
	}
	if acc == nil {
		return operation_setting.GetLoanSetting().CreditInitial, nil
	}
	return acc.CreditScore, nil
}
