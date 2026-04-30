package minimax

import "strings"

// https://platform.minimaxi.com/docs/api-reference/api-overview

var ModelList = []string{
	"MiniMax-M2.7",
	"MiniMax-M2.7-highspeed",
	"MiniMax-M2.5",
	"MiniMax-M2.5-highspeed",
	"MiniMax-M2.1",
	"MiniMax-M2.1-highspeed",
	"MiniMax-M2",
	"speech-2.8-hd",
	"speech-2.8-turbo",
	"speech-2.6-hd",
	"speech-2.6-turbo",
	"speech-02-hd",
	"speech-02-turbo",
	"speech-01-hd",
	"speech-01-turbo",
	"image-01",
	"image-01-live",
	"music-2.6",
	"music-cover",
	"music-2.6-free",
	"music-cover-free",
	"MiniMax-Hailuo-2.3",
	"MiniMax-Hailuo-2.3-Fast",
	"MiniMax-Hailuo-02",
	"T2V-01-Director",
	"T2V-01",
	"I2V-01-Director",
	"I2V-01-live",
	"I2V-01",
	"S2V-01",
}

var ChannelName = "minimax"

func isMiniMaxMusicModel(model string) bool {
	return strings.HasPrefix(model, "music-")
}
