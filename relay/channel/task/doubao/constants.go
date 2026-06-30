package doubao

import "strings"

var ModelList = []string{
	"doubao-seedance-1-0-pro-250528",
	"doubao-seedance-1-0-lite-t2v-250428",
	"doubao-seedance-1-0-lite-i2v-250428",
	"doubao-seedance-1-0-pro-fast-251015",
	"doubao-seedance-1-5-pro-251215",
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-fast-260128",
}

var ChannelName = "doubao-video"

type videoPriceKey struct {
	is1080p  bool
	is4k     bool
	hasVideo bool
}

// videoPriceTable records Doubao Seedance 2.0 prices relative to the default
// 480p/720p text-to-video price configured in ModelRatio.
var videoPriceTable = map[string]map[videoPriceKey]float64{
	"doubao-seedance-2-0-260128": {
		{hasVideo: false}:                46.0,
		{hasVideo: true}:                 28.0,
		{is1080p: true, hasVideo: false}: 51.0,
		{is1080p: true, hasVideo: true}:  31.0,
		{is4k: true, hasVideo: false}:    26.0,
		{is4k: true, hasVideo: true}:     16.0,
	},
	"doubao-seedance-2-0-fast-260128": {
		{hasVideo: false}: 37.0,
		{hasVideo: true}:  22.0,
	},
}

func GetVideoInputRatio(modelName, resolution string, hasVideo bool) (float64, bool) {
	prices, ok := videoPriceTable[modelName]
	if !ok {
		return 1.0, false
	}
	basePrice, ok := prices[videoPriceKey{hasVideo: false}]
	if !ok || basePrice == 0 {
		return 1.0, false
	}
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	price, ok := prices[videoPriceKey{
		is1080p:  resolution == "1080p",
		is4k:     resolution == "4k",
		hasVideo: hasVideo,
	}]
	if !ok {
		// Missing combinations, such as fast 1080p/4K, fall back to base billing.
		return 1.0, true
	}
	return price / basePrice, true
}
