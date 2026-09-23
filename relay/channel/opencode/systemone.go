package opencode

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func (a *Adaptor) validateRequest(info *relaycommon.RelayInfo) error {
	if info == nil || info.ChannelMeta == nil {
		return nil
	}
	if a.RequestMode == requestModeSystemOne {
		if IsGoBase(info.ChannelBaseUrl) {
			return errors.New("the /v1/systemone endpoint is not supported by this base URL")
		}
		if info.IsStream {
			return errors.New("the /v1/systemone endpoint does not support streaming")
		}
		return nil
	}
	model := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(info.UpstreamModelName, "models/")))
	if stringListContains(constant.OpenCodeZenSystemOneModels, model) {
		return errors.New("this model requires the native /v1/systemone endpoint")
	}
	return nil
}
