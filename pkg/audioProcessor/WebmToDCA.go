package audioProcessor

import (
	"encoding/binary"
	"io"
	"time"

	"github.com/remko/go-mkvparse"
)

func (p *MyParser) HandleBinary(id mkvparse.ElementID, value []byte, _ mkvparse.ElementInfo) error {
	switch id {
	case mkvparse.SimpleBlockElement:
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(len(value))-4)
		var opus []byte
		opus = append(opus, buf...)
		opus = append(opus, value[4:]...)
		p.w.Write(opus)
	default:
	}
	return nil
}

type MyParser struct {
	w io.WriteCloser
}

func (p *MyParser) HandleMasterBegin(_ mkvparse.ElementID, _ mkvparse.ElementInfo) (bool, error) {
	return true, nil
}

func (p *MyParser) HandleMasterEnd(_ mkvparse.ElementID, _ mkvparse.ElementInfo) error {
	return nil
}

func (p *MyParser) HandleString(_ mkvparse.ElementID, _ string, _ mkvparse.ElementInfo) error {
	return nil
}

func (p *MyParser) HandleInteger(_ mkvparse.ElementID, _ int64, _ mkvparse.ElementInfo) error {
	return nil
}

func (p *MyParser) HandleFloat(_ mkvparse.ElementID, _ float64, _ mkvparse.ElementInfo) error {
	return nil
}

func (p *MyParser) HandleDate(_ mkvparse.ElementID, _ time.Time, _ mkvparse.ElementInfo) error {
	return nil
}

func convertWebmToDca(audio io.ReadCloser, out io.WriteCloser) error {
	handler := MyParser{w: out}
	err := mkvparse.Parse(audio, &handler)
	if err != nil {
		return err
	}
	return nil
}
