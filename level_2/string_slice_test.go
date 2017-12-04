package test

import (
	"testing"
	"github.com/stretchr/testify/require"
	"github.com/esdb/gocodec"
	"github.com/json-iterator/go"
	"encoding/json"
)

func Test_string_slice(t *testing.T) {
	should := require.New(t)
	encoded, err := gocodec.Marshal([]string{"h", "i"})
	should.Nil(err)
	should.Equal([]byte{
		24, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, // sliceHeader
		32, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0,                         // string header
		17, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0,                         // string header
		'h', 'i'}, encoded)
	var val []string
	should.Nil(gocodec.Unmarshal(encoded, &val))
	should.Equal([]string{"h", "i"}, val)
}

func Benchmark_string_slice(b *testing.B) {
	data := []string{"hello", "world"}
	gocEncoded, _ := gocodec.Marshal(data)
	jsonEncoded, _ := jsoniter.Marshal(data)
	b.Run("goc encode", func(b *testing.B) {
		b.ReportAllocs()
		gocEncoder := gocodec.DefaultConfig.NewGocEncoder(nil)
		for i := 0; i < b.N; i++ {
			gocEncoder.Reset(gocEncoder.Buffer()[:0])
			gocEncoder.EncodeVal(data)
		}
	})
	b.Run("goc decode", func(b *testing.B) {
		b.ReportAllocs()
		gocDecoder := gocodec.DefaultConfig.NewGocDecoder(nil)
		for i := 0; i < b.N; i++ {
			gocDecoder.Reset(append(([]byte)(nil), gocEncoded...))
			gocDecoder.DecodeVal(&data)
		}
	})
	b.Run("json encode", func(b *testing.B) {
		b.ReportAllocs()
		jsonEncoder := jsoniter.ConfigFastest.BorrowStream(nil)
		for i := 0; i < b.N; i++ {
			jsonEncoder.Reset(nil)
			jsonEncoder.WriteVal(data)
		}
	})
	b.Run("json decode", func(b *testing.B) {
		b.ReportAllocs()
		jsonDecoder := jsoniter.ConfigFastest.BorrowIterator(nil)
		for i := 0; i < b.N; i++ {
			jsonDecoder.ResetBytes(jsonEncoded)
			jsonDecoder.ReadVal(&data)
		}
	})
	b.Run("encoding/json decode", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			json.Unmarshal(jsonEncoded, &data)
		}
	})
}
