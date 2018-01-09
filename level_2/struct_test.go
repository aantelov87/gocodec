package test

import (
	"testing"
	"github.com/esdb/gocodec"
	"github.com/stretchr/testify/require"
)

func Test_nil_struct_within_struct(t *testing.T) {
	should := require.New(t)
	type SubObject struct {
		length uint
		set    []uint64
	}
	type TestObject struct {
		f1 uint
		f2 uint
		f3 *SubObject
	}
	obj := TestObject{}
	encoded, err := gocodec.Marshal(obj)
	should.Nil(err)
	decoded, err := gocodec.ReadonlyConfig.Unmarshal(encoded, (*TestObject)(nil))
	should.Nil(err)
	should.Equal(obj, *decoded.(*TestObject))
	decoded, err = gocodec.Unmarshal(encoded, (*TestObject)(nil))
	should.Nil(err)
	should.Equal(obj, *decoded.(*TestObject))
}

func Test_struct_within_struct(t *testing.T) {
	should := require.New(t)
	type SubObject struct {
		length uint
		set    []uint64
	}
	type TestObject struct {
		f1 uint
		f2 uint
		f3 *SubObject
	}
	obj := TestObject{f1: 1, f2: 2, f3: &SubObject{length: 3, set: []uint64{100}}}
	encoded, err := gocodec.Marshal(obj)
	should.Nil(err)
	should.Equal([]byte{
		0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x2, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x30, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x3, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x50, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x64, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
	}, encoded[24:])
	decoded, err := gocodec.ReadonlyConfig.Unmarshal(encoded, (*TestObject)(nil))
	should.Nil(err)
	should.Equal(obj, *decoded.(*TestObject))
	decoded, err = gocodec.Unmarshal(encoded, (*TestObject)(nil))
	should.Nil(err)
	should.Equal(obj, *decoded.(*TestObject))
}

func Test_multiple_struct(t *testing.T) {
	should := require.New(t)
	type TestObj struct {
		Field1 []int
		Field2 [][]byte
	}
	stream := gocodec.NewStream(nil)
	stream.Marshal(TestObj{Field2:[][]byte{[]byte("hello")}})
	stream.Marshal(TestObj{Field2:[][]byte{[]byte("world")}})
	should.Nil(stream.Error)
	iter := gocodec.NewIterator(stream.Buffer())
	obj := iter.Unmarshal((*TestObj)(nil))
	should.Nil(iter.Error)
	should.Equal([][]byte{[]byte("hello")}, obj.(*TestObj).Field2)
	obj = iter.Unmarshal((*TestObj)(nil))
	should.Nil(iter.Error)
	should.Equal([][]byte{[]byte("world")}, obj.(*TestObj).Field2)
}