package service

import (
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestMiniMaxCodeModelSnapshotManagedModelIDs(t *testing.T) {
	var snapshot miniMaxCodeModelSnapshot
	raw := `{"version":"1.0","ttlSeconds":300,"providers":[
		{"providerId":"minimax_api","config":{"models":{"MiniMax-API-Only":{}}}},
		{"providerId":"minimax","config":{
			"model_order":["MiniMax-M3.1-Flash","MiniMax-Gone","MiniMax-M3"],
			"models":{
				"MiniMax-M3":{},
				"MiniMax-M3.1-Flash":{"limit":{"context":512000}},
				"MiniMax-Old":{"enabled":false},
				"MiniMax-Extra-B":{},
				"MiniMax-Extra-A":{"enabled":true}
			}
		}}
	]}`
	if err := common.UnmarshalJsonStr(raw, &snapshot); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ids, err := snapshot.managedModelIDs()
	if err != nil {
		t.Fatalf("managedModelIDs: %v", err)
	}
	want := []string{"MiniMax-M3.1-Flash", "MiniMax-M3", "MiniMax-Extra-A", "MiniMax-Extra-B"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestMiniMaxCodeModelSnapshotWithoutManagedProviderIsError(t *testing.T) {
	var snapshot miniMaxCodeModelSnapshot
	raw := `{"providers":[{"providerId":"minimax","config":{"models":{"MiniMax-Old":{"enabled":false}}}}]}`
	if err := common.UnmarshalJsonStr(raw, &snapshot); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, err := snapshot.managedModelIDs(); err == nil {
		t.Fatal("snapshot without enabled managed models should be an error")
	}
}
