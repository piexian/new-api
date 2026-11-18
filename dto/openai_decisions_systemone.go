package dto

import (
	"encoding/json"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// SystemOneAnswersFromOpenAIDecisions 把 OpenAI Decisions answers 数组按 name 回填为
// SystemOne 映射表:predicate→noul(probability→noul,confidence=max(p,1-p))、
// choice/score 字段同构重排,refusal 等未知类型按名原样透传。
func SystemOneAnswersFromOpenAIDecisions(answers []json.RawMessage) map[string]json.RawMessage {
	if len(answers) == 0 {
		return nil
	}
	converted := make(map[string]json.RawMessage, len(answers))
	for _, answer := range answers {
		var envelope struct {
			Type          string          `json:"type"`
			Name          *string         `json:"name"`
			Probability   *float64        `json:"probability"`
			Choice        json.RawMessage `json:"choice"`
			Confidence    *float64        `json:"confidence"`
			Probabilities []struct {
				Value       json.RawMessage `json:"value"`
				Probability float64         `json:"probability"`
			} `json:"probabilities"`
			Score *float64 `json:"score"`
		}
		if err := common.Unmarshal(answer, &envelope); err != nil {
			continue
		}
		if envelope.Name == nil {
			continue
		}
		switch envelope.Type {
		case "predicate":
			if envelope.Probability == nil {
				continue
			}
			probability := *envelope.Probability
			confidence := probability
			if 1-probability > confidence {
				confidence = 1 - probability
			}
			converted[*envelope.Name], _ = common.Marshal(map[string]any{
				"type":       "noul",
				"noul":       probability,
				"confidence": confidence,
			})
		case "choice":
			entry := map[string]any{"type": "choice"}
			if len(envelope.Choice) > 0 {
				entry["choice"] = json.RawMessage(envelope.Choice)
			}
			if envelope.Confidence != nil {
				entry["confidence"] = *envelope.Confidence
			}
			probabilities := make(map[string]float64, len(envelope.Probabilities))
			for _, item := range envelope.Probabilities {
				var value any
				if err := common.Unmarshal(item.Value, &value); err == nil && value != nil {
					probabilities[fmt.Sprintf("%v", value)] = item.Probability
				}
			}
			if len(probabilities) > 0 {
				entry["probabilities"] = probabilities
			}
			converted[*envelope.Name], _ = common.Marshal(entry)
		case "score":
			entry := map[string]any{"type": "score"}
			if envelope.Score != nil {
				entry["score"] = *envelope.Score
			}
			if envelope.Confidence != nil {
				entry["confidence"] = *envelope.Confidence
			}
			probabilities := make(map[string]float64, len(envelope.Probabilities))
			for _, item := range envelope.Probabilities {
				var level int64
				if err := common.Unmarshal(item.Value, &level); err == nil {
					probabilities[fmt.Sprintf("%d", level)] = item.Probability
				}
			}
			if len(probabilities) > 0 {
				entry["probabilities"] = probabilities
			}
			converted[*envelope.Name], _ = common.Marshal(entry)
		default:
			converted[*envelope.Name] = answer
		}
	}
	return converted
}
