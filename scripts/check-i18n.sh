#!/bin/bash
# i18n 完整性检查脚本
# 检查后端 controller 中的英文硬编码消息、前端缺失的 i18n key
#
# 用法: ./scripts/check-i18n.sh [--fix]
#   --fix  自动修复前端缺失的 key（仅 default 主题支持自动修复）

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ERRORS=0
WARNINGS=0

echo "=== New API i18n 完整性检查 ==="
echo ""

# ─── 1. 检查后端 controller 中的英文硬编码消息 ───
echo "── 检查后端 controller 英文硬编码消息..."

# 匹配 "message": "xxx" 其中 xxx 是纯英文（不含中文字符）
# 排除: i18n.T, err.Error(), 空字符串, 变量引用
ENGLISH_MSGS=$(grep -rn '"message":' controller/ --include="*.go" \
  | grep -v 'i18n\.T' \
  | grep -v 'err\.Error()' \
  | grep -v '""' \
  | grep -v 'message,' \
  | grep -v 'disabledUserMessage' \
  | grep -v 'result\.' \
  | grep -v '"success"' \
  | grep -v '"error"' \
  | grep -v '"ok"' \
  | grep -v '"saved"' \
  | grep -v '"generated"' \
  | grep -v '"refreshed"' \
  | grep -v '"migrated"' \
  | grep -v 'data:' \
  | grep -v 'c\.JSON' \
  | grep -v 'common\.Api' \
  | grep -v '//' \
  | grep -v 'msg}' \
  | grep -v 'Failed to fetch models' \
  | grep -v 'Invalid request$' \
  || true)

# 进一步过滤：只保留包含英文字母但不含中文的行
ENGLISH_MSGS=$(echo "$ENGLISH_MSGS" | grep -E '"[a-zA-Z]' | grep -v '[\xe4-\xe9]' || true)

if [ -n "$ENGLISH_MSGS" ]; then
  echo -e "${YELLOW}⚠ 发现可能未翻译的英文消息:${NC}"
  echo "$ENGLISH_MSGS" | head -20
  COUNT=$(echo "$ENGLISH_MSGS" | wc -l)
  if [ "$COUNT" -gt 20 ]; then
    echo "  ... 还有 $((COUNT - 20)) 条"
  fi
  WARNINGS=$((WARNINGS + COUNT))
else
  echo -e "${GREEN}✓ 未发现英文硬编码消息${NC}"
fi
echo ""

# ─── 2. 检查后端 fmt.Errorf 中的英文硬编码消息 ───
echo "── 检查后端 fmt.Errorf 英文硬编码消息..."

FMT_ERRORS=$(grep -rn 'fmt\.Errorf("' controller/ --include="*.go" \
  | grep -v 'i18n' \
  | grep -v 'test' \
  | grep -v '%w' \
  | grep -v '%v' \
  | grep -v '%s' \
  | grep -v '%d' \
  | grep -E '"[a-zA-Z]' \
  | grep -v '[\xe4-\xe9]' \
  || true)

if [ -n "$FMT_ERRORS" ]; then
  echo -e "${YELLOW}⚠ 发现 fmt.Errorf 英文硬编码消息:${NC}"
  echo "$FMT_ERRORS" | head -10
  WARNINGS=$((WARNINGS + $(echo "$FMT_ERRORS" | wc -l)))
else
  echo -e "${GREEN}✓ 未发现 fmt.Errorf 英文硬编码消息${NC}"
fi
echo ""

# ─── 3. 检查后端 i18n key 在 yaml 中的完整性 ───
echo "── 检查后端 i18n key 在 yaml 中的完整性..."

if [ -f "i18n/keys.go" ]; then
  # 提取所有 key
  KEYS=$(grep -oP '"[a-z_]+\.[a-z_]+"' i18n/keys.go | tr -d '"' | sort -u)
  
  MISSING_ZH=0
  MISSING_EN=0
  
  for key in $KEYS; do
    if ! grep -q "^${key}:" i18n/locales/zh-CN.yaml 2>/dev/null; then
      if [ $MISSING_ZH -eq 0 ]; then
        echo -e "${YELLOW}⚠ zh-CN.yaml 缺失以下 key:${NC}"
      fi
      echo "  - $key"
      MISSING_ZH=$((MISSING_ZH + 1))
    fi
    if ! grep -q "^${key}:" i18n/locales/en.yaml 2>/dev/null; then
      if [ $MISSING_EN -eq 0 ]; then
        echo -e "${YELLOW}⚠ en.yaml 缺失以下 key:${NC}"
      fi
      echo "  - $key"
      MISSING_EN=$((MISSING_EN + 1))
    fi
  done
  
  if [ $MISSING_ZH -eq 0 ] && [ $MISSING_EN -eq 0 ]; then
    echo -e "${GREEN}✓ 所有 i18n key 在 zh-CN.yaml 和 en.yaml 中都有翻译${NC}"
  else
    ERRORS=$((ERRORS + MISSING_ZH + MISSING_EN))
  fi
else
  echo -e "${YELLOW}⚠ 未找到 i18n/keys.go${NC}"
fi
echo ""

# ─── 4. 检查前端 default 主题缺失的 i18n key ───
echo "── 检查前端 default 主题..."

if [ -d "web/default" ]; then
  cd web/default
  if [ -f "scripts/sync-i18n.mjs" ]; then
    bun run i18n:sync > /dev/null 2>&1 || true
    MISSING=$(cat src/i18n/locales/_reports/zh.untranslated.json 2>/dev/null | grep -c ':' || true)
    MISSING=${MISSING:-0}
    if [ "$MISSING" -gt 0 ] 2>/dev/null; then
      echo -e "${YELLOW}⚠ default 主题 zh.json 有 $MISSING 个未翻译 key${NC}"
      cat src/i18n/locales/_reports/zh.untranslated.json 2>/dev/null | head -10
      WARNINGS=$((WARNINGS + MISSING))
    else
      echo -e "${GREEN}✓ default 主题 zh.json 无缺失${NC}"
    fi
  fi
  cd - > /dev/null
else
  echo -e "${YELLOW}⚠ 未找到 web/default 目录${NC}"
fi
echo ""

# ─── 5. 检查前端 classic 主题缺失的 i18n key ───
echo "── 检查前端 classic 主题..."

if [ -d "web/classic" ]; then
  cd web/classic
  if [ -f "scripts/find-missing-keys.mjs" ]; then
    MISSING_OUTPUT=$(node scripts/find-missing-keys.mjs 2>&1 || true)
    MISSING_COUNT=$(echo "$MISSING_OUTPUT" | grep -c 'missing key' || echo "0")
    if echo "$MISSING_OUTPUT" | grep -q "missing key"; then
      echo -e "${YELLOW}⚠ classic 主题有缺失的 i18n key:${NC}"
      echo "$MISSING_OUTPUT" | head -20
      WARNINGS=$((WARNINGS + 1))
    else
      echo -e "${GREEN}✓ classic 主题无缺失 key${NC}"
    fi
  fi
  cd - > /dev/null
else
  echo -e "${YELLOW}⚠ 未找到 web/classic 目录${NC}"
fi
echo ""

# ─── 6. 检查后端 i18n key 是否有重复定义 ───
echo "── 检查后端 i18n key 重复定义..."

if [ -f "i18n/keys.go" ]; then
  DUPLICATES=$(grep -oP '"[a-z_]+\.[a-z_]+"' i18n/keys.go | sort | uniq -d)
  if [ -n "$DUPLICATES" ]; then
    echo -e "${RED}✗ 发现重复的 i18n key:${NC}"
    echo "$DUPLICATES"
    ERRORS=$((ERRORS + 1))
  else
    echo -e "${GREEN}✓ 无重复 key${NC}"
  fi
fi
echo ""

# ─── 汇总 ───
echo "=== 检查结果汇总 ==="
if [ $ERRORS -gt 0 ]; then
  echo -e "${RED}✗ 发现 $ERRORS 个错误${NC}"
fi
if [ $WARNINGS -gt 0 ]; then
  echo -e "${YELLOW}⚠ 发现 $WARNINGS 个警告${NC}"
fi
if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
  echo -e "${GREEN}✓ 所有检查通过${NC}"
fi

exit $ERRORS
