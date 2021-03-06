package test

import (
	"testing"
	"github.com/stretchr/testify/require"
	"github.com/esdb/gocodec"
)

func Test_int_slice(t *testing.T) {
	should := require.New(t)
	encoded, err := gocodec.Marshal([]int{1, 2, 3})
	should.Nil(err)
	should.Equal([]byte{
		0x18, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0,
		1, 0, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		3, 0, 0, 0, 0, 0, 0, 0}, encoded[8:])
	decoded, err := gocodec.ReadonlyConfig.Unmarshal(encoded, (*[]int)(nil))
	should.Equal([]int{1, 2, 3}, *decoded.(*[]int))
	decoded, err = gocodec.Unmarshal(encoded, (*[]int)(nil))
	should.Equal([]int{1, 2, 3}, *decoded.(*[]int))
}
