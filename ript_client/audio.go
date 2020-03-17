package main

import (
	"fmt"

	"github.com/gordonklaus/portaudio"
	"gopkg.in/hraban/opus.v2"
)

const (
	sampleRate      = 48000
	readBufferSize  = 4800
	writeBufferSize = 4800

	audioChannels       = 1
	opusFrameMs         = 60
	opusFrameSamples    = sampleRate / 1000 * opusFrameMs
	opusFrameBufferSize = 480
)

func packFrames(frames [][]byte) []byte {
	size := 2 * len(frames)
	for _, frame := range frames {
		size += len(frame)
	}

	data := make([]byte, size)
	offset := 0
	for _, frame := range frames {
		size := len(frame)
		data[offset] = byte(size >> 8)
		data[offset+1] = byte(size)
		copy(data[offset+2:], frame)
		offset += size + 2
	}

	return data
}

func unpackFrames(data []byte) ([][]byte, error) {
	offset := 0
	frames := [][]byte{}
	for offset < len(data) {
		size := (int(data[offset]) << 8) + int(data[offset+1])
		if size > len(data)-offset-2 {
			return nil, fmt.Errorf("Unpack error")
		}
		frames = append(frames, data[offset+2:offset+2+size])
		offset += size + 2
	}

	return frames, nil
}

func opusCompress(samples []int16) ([]byte, error) {
	enc, err := opus.NewEncoder(sampleRate, audioChannels, opus.AppVoIP)
	if err != nil {
		return nil, err
	}

	nFrames := len(samples) / opusFrameSamples
	frames := make([][]byte, nFrames)
	for i := range frames {
		offset := i * opusFrameSamples
		pcm := samples[offset : offset+opusFrameSamples]

		frames[i] = make([]byte, opusFrameBufferSize)
		n, err := enc.Encode(pcm, frames[i])
		if err != nil {
			return nil, err
		}

		frames[i] = frames[i][:n]
	}

	return packFrames(frames), nil
}

func opusDecompress(data []byte) ([]int16, error) {
	frames, err := unpackFrames(data)
	if err != nil {
		return nil, err
	}

	dec, err := opus.NewDecoder(sampleRate, audioChannels)
	if err != nil {
		return nil, err
	}

	nFrames := len(frames)
	samples := make([]int16, nFrames*opusFrameSamples)
	for i := range frames {
		offset := i * opusFrameSamples
		pcm := samples[offset : offset+opusFrameSamples]

		n, err := dec.Decode(frames[i], pcm)
		if err != nil {
			return nil, err
		}
		if n != opusFrameSamples {
			return nil, fmt.Errorf("Wrong number of samples: %d != %d", n, opusFrameSamples)
		}
	}

	return samples, nil
}

type Microphone struct {
	stream     *portaudio.Stream
	errChan    chan error
	stopChan   chan bool
	doneChan   chan bool
	contentChan chan []byte
	readBuffer []int16
	read       []int16
}

func NewMicrophone() (*Microphone, error) {
	m := &Microphone{
		errChan:    make(chan error, 1),
		stopChan:   make(chan bool, 1),
		doneChan:   make(chan bool, 1),
		readBuffer: make([]int16, readBufferSize),
		read:       []int16{},
	}

	var err error
	m.stream, err = portaudio.OpenDefaultStream(1, 0, sampleRate, readBufferSize, m.readBuffer)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Microphone) setContentChan(content chan []byte) {
	m.contentChan = content
}

func (m *Microphone) readStream() {
	defer func() {
		m.doneChan <- true
	}()

	for {
		select {
		case <-m.stopChan:
			m.stream.Stop()
			return
		default:
			err := m.stream.Read()
			if err != nil {
				m.errChan <- err
				return
			}

			opus, err := opusCompress(m.readBuffer)
			if err != nil {
				m.errChan <- err
				return
			}
			m.contentChan <- opus
			//m.read = append(m.read, m.readBuffer...)
		}
	}
}

func (m *Microphone) Start() error {
	err := m.stream.Start()
	if err != nil {
		return err
	}

	go m.readStream()
	return nil
}

func (m *Microphone) Stop() ([]byte, error) {
	select {
	case err := <-m.errChan:
		return nil, err
	default:
	}

	m.stopChan <- true
	<-m.doneChan
	/*opus, err := opusCompress(m.read)
	if err != nil {
		return nil, err
	}*/

	m.read = m.read[:0]
	return nil, nil
}

func (m *Microphone) Close() error {
	return m.stream.Close()
}

type Speaker struct {
	writeBuffer []int16
	stream      *portaudio.Stream
}

func NewSpeaker() (*Speaker, error) {
	s := &Speaker{
		writeBuffer: make([]int16, writeBufferSize),
	}
	var err error
	s.stream, err = portaudio.OpenDefaultStream(0, 1, sampleRate, writeBufferSize, s.writeBuffer)
	if err != nil {
		return nil, err
	}

	err = s.stream.Start()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Speaker) Play(opus []byte) error {
	clip, err := opusDecompress(opus)
	if err != nil {
		return err
	}

	buffer := make([]int16, len(clip))
	copy(buffer, clip)
	for len(buffer) > 0 {
		if len(s.writeBuffer) > len(buffer) {
			s.writeBuffer = s.writeBuffer[:len(buffer)]
		}
		copy(s.writeBuffer, buffer)
		buffer = buffer[len(s.writeBuffer):]

		err := s.stream.Write()
		if err != nil {
			return err
		}
	}

	return nil
}
