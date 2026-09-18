package stepfun

import "github.com/QuantumNous/new-api/constant"

var ChannelName = "stepfun"

// ModelList 渠道"填入所有模型"预置清单：上游 GET /v1/models 快照 + 路由用伪模型。
var ModelList = append(append([]string{}, constant.StepFunListedModels...), constant.StepFunPseudoModels...)
