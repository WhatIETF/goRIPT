package api



type AdvertismentInfo struct {

}

// todo: support codec params
type CodecInfo struct {
	Codec string
}

type SourceCapability struct {
	SourceId int
	Codecs []CodecInfo
}

type SinkCapability struct {
	SinkId int

}

func (ad Advertisement) parse() {

}
