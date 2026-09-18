package zhipu_4v

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/google/uuid"
)

// ZCode 客户端版本对齐 ZCode Desktop 3.12.3 内置常量（X-Client-Version /
// X-ZCode-App-Version / UA 同源）。服务端 forceUpdate.minimalVersion=3.5.3
// 仅为强制更新下限，指纹取当前最新版而非下限。
const zcodeClientVersion = "3.12.3"

// zcodeOsVersion 与 X-Platform=win32-x64 配套（Windows 11 24H2，对齐官方
// os.release() 在 Windows 上的形态）；不使用宿主机 release，避免与平台指纹矛盾。
const zcodeOsVersion = "10.0.26100"

// setupZCodeTraceHeaders 清除 Claude 系客户端指纹头并写入 ZCode 请求级
// tracing 头。Coding Plan 渠道的默认（非 ZCode 模式）行为。
func setupZCodeTraceHeaders(req *http.Header) {
	if req == nil {
		return
	}
	for _, name := range zcodeReplacedClaudeHeaders {
		req.Del(name)
	}
	req.Set("x-request-id", uuid.NewString())
	req.Set("x-zcode-session-type", "main")
	req.Set("x-zcode-trace-id", uuid.NewString())
	req.Set("x-query-id", uuid.NewString())
	req.Set("x-session-id", uuid.NewString())
}

var (
	zcodeDeviceMidOnce sync.Once
	zcodeDeviceMidVal  string
)

// zcodeDeviceMid 返回 ZCode 设备指纹 X-Device-Mid：官方为安装级持久随机
// UUID（telemetry-state.json），此处取宿主机 machine-id 作为"服务器自己的"
// 持久设备 ID，格式化为 UUID；读取失败时退化为进程内随机 UUID（仅本进程稳定）。
func zcodeDeviceMid() string {
	zcodeDeviceMidOnce.Do(func() {
		for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
			raw, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			id := strings.ToLower(strings.TrimSpace(string(raw)))
			if len(id) == 32 && isHexString(id) {
				zcodeDeviceMidVal = id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:32]
				return
			}
		}
		zcodeDeviceMidVal = uuid.NewString()
	})
	return zcodeDeviceMidVal
}

func isHexString(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return len(s) > 0
}

// zcodeSessionId 派生稳定 x-session-id：官方为会话级持久 ID（剥 sess_ 前缀），
// 服务端按 设备 + 渠道密钥 + 令牌 确定性派生，同一客户端身份跨请求稳定，
// metadata.user_id 与签名共用同一值。
func zcodeSessionId(info *relaycommon.RelayInfo) string {
	seed := "zcode-session\n" + zcodeDeviceMid()
	if info != nil {
		seed += "\n" + info.ApiKey
		if info.TokenId > 0 {
			seed += "\n" + strconv.Itoa(info.TokenId)
		}
	}
	sum := sha256.Sum256([]byte(seed))
	hexID := hex.EncodeToString(sum[:])[:32]
	return hexID[0:8] + "-" + hexID[8:12] + "-" + hexID[12:16] + "-" + hexID[16:20] + "-" + hexID[20:32]
}

// setupZCodeCompatibilityHeaders 在 tracing 头之上补齐 ZCode 桌面端
// production 设备指纹。GLM Coding Plan 网关按 ZCode 指纹发放客户端权益
// （夜间活动 0 扣费、全天 1.5× 加成），仅在渠道开启 ZCode 模式时使用。
func setupZCodeCompatibilityHeaders(req *http.Header, info *relaycommon.RelayInfo) {
	if req == nil {
		return
	}
	setupZCodeTraceHeaders(req)
	req.Set("User-Agent", "ZCode/"+zcodeClientVersion)
	req.Set("HTTP-Referer", "https://zcode.z.ai")
	req.Set("X-Title", "Z Code@electron")
	req.Set("X-ZCode-App-Version", zcodeClientVersion)
	req.Set("X-Platform", "win32-x64")
	req.Set("X-Release-Channel", "production")
	req.Set("X-Client-Language", "zh-CN")
	req.Set("X-Client-Timezone", "Asia/Shanghai")
	req.Set("X-Os-Category", "windows")
	// 3.12.3 官方 host 附带项：OS 版本 + 持久设备 ID；x-session-id 覆盖为
	// 稳定派生值（官方为会话级持久 ID，非每请求随机）。
	req.Set("X-Os-Version", zcodeOsVersion)
	req.Set("X-Device-Mid", zcodeDeviceMid())
	req.Set("x-session-id", zcodeSessionId(info))
}

var zcodeReplacedClaudeHeaders = []string{
	"User-Agent",
	"X-Stainless-Retry-Count",
	"X-Stainless-Runtime-Version",
	"X-Stainless-Package-Version",
	"X-Stainless-Runtime",
	"X-Stainless-Lang",
	"X-Stainless-Arch",
	"X-Stainless-OS",
	"X-Stainless-Timeout",
	"anthropic-client-platform",
	"anthropic-client-version",
	"anthropic-dangerous-direct-browser-access",
	"x-app",
	"X-Claude-Code-Session-Id",
	"x-client-request-id",
}
