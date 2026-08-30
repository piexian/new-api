package agnes

import agneschannel "github.com/QuantumNous/new-api/relay/channel/agnes"

const (
	ChannelName       = agneschannel.ChannelName
	ModelVideoV20     = agneschannel.ModelVideoV20
	ModelVideo25Flash = agneschannel.ModelVideo25Flash
)

var ModelList = []string{
	ModelVideoV20,
	ModelVideo25Flash,
}
