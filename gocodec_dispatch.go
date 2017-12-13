package gocodec

import (
	"reflect"
	"sync/atomic"
	"unsafe"
	"fmt"
)

func (cfg *frozenConfig) addDecoderToCache(cacheKey reflect.Type, decoder ValDecoder) {
	done := false
	for !done {
		ptr := atomic.LoadPointer(&cfg.decoderCache)
		cache := *(*map[reflect.Type]ValDecoder)(ptr)
		copied := map[reflect.Type]ValDecoder{}
		for k, v := range cache {
			copied[k] = v
		}
		copied[cacheKey] = decoder
		done = atomic.CompareAndSwapPointer(&cfg.decoderCache, ptr, unsafe.Pointer(&copied))
	}
}

func (cfg *frozenConfig) addEncoderToCache(cacheKey reflect.Type, encoder ValEncoder) {
	done := false
	for !done {
		ptr := atomic.LoadPointer(&cfg.encoderCache)
		cache := *(*map[reflect.Type]ValEncoder)(ptr)
		copied := map[reflect.Type]ValEncoder{}
		for k, v := range cache {
			copied[k] = v
		}
		copied[cacheKey] = encoder
		done = atomic.CompareAndSwapPointer(&cfg.encoderCache, ptr, unsafe.Pointer(&copied))
	}
}

func (cfg *frozenConfig) getDecoderFromCache(cacheKey reflect.Type) ValDecoder {
	ptr := atomic.LoadPointer(&cfg.decoderCache)
	cache := *(*map[reflect.Type]ValDecoder)(ptr)
	return cache[cacheKey]
}

func (cfg *frozenConfig) getEncoderFromCache(cacheKey reflect.Type) ValEncoder {
	ptr := atomic.LoadPointer(&cfg.encoderCache)
	cache := *(*map[reflect.Type]ValEncoder)(ptr)
	return cache[cacheKey]
}

func encoderOfType(cfg *frozenConfig, valType reflect.Type) (ValEncoder, error) {
	cacheKey := valType
	encoder := cfg.getEncoderFromCache(cacheKey)
	if encoder != nil {
		return encoder, nil
	}
	encoder, err := createEncoderOfType(cfg, valType)
	if err != nil {
		return nil, err
	}
	cfg.addEncoderToCache(cacheKey, encoder)
	return encoder, err
}

func decoderOfType(cfg *frozenConfig, valType reflect.Type) (ValDecoder, error) {
	cacheKey := valType
	decoder := cfg.getDecoderFromCache(cacheKey)
	if decoder != nil {
		return decoder, nil
	}
	decoder, err := createDecoderOfType(cfg, valType)
	if err != nil {
		return nil, err
	}
	cfg.addDecoderToCache(cacheKey, decoder)
	return decoder, err
}

func createEncoderOfType(cfg *frozenConfig, valType reflect.Type) (ValEncoder, error) {
	valKind := valType.Kind()
	switch valKind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Uintptr, reflect.Float32, reflect.Float64:
		return &NoopCodec{BaseCodec: *NewBaseCodec(valType, uint32(valKind))}, nil
	case reflect.String:
		return &stringCodec{BaseCodec: *NewBaseCodec(valType, uint32(valKind))}, nil
	case reflect.Struct:
		signature := uint32(valKind)
		fields := make([]structFieldEncoder, 0, valType.NumField())
		for i := 0; i < valType.NumField(); i++ {
			encoder, err := createEncoderOfType(cfg, valType.Field(i).Type)
			if err != nil {
				return nil, err
			}
			signature = 31 * signature + encoder.Signature()
			if !encoder.IsNoop() {
				fields = append(fields, structFieldEncoder{
					offset:  valType.Field(i).Offset,
					encoder: encoder,
				})
			}
		}
		encoder := &structEncoder{BaseCodec: *NewBaseCodec(valType, signature), fields: fields}
		if len(fields) == 1 && valType.Field(0).Type.Kind() == reflect.Ptr {
			return &singlePointerFix{ValEncoder: encoder}, nil
		}
		return encoder, nil
	case reflect.Slice:
		signature := uint32(valKind)
		elemEncoder, err := createEncoderOfType(cfg, valType.Elem())
		if err != nil {
			return nil, err
		}
		signature = 31 * signature + elemEncoder.Signature()
		if elemEncoder.IsNoop() {
			elemEncoder = nil
		}
		return &sliceEncoder{BaseCodec: *NewBaseCodec(valType, signature),
			elemSize: int(valType.Elem().Size()), elemEncoder: elemEncoder}, nil
	case reflect.Ptr:
		signature := uint32(valKind)
		elemEncoder, err := createEncoderOfType(cfg, valType.Elem())
		if err != nil {
			return nil, err
		}
		signature = 31 * signature + elemEncoder.Signature()
		encoder := &pointerEncoder{BaseCodec: *NewBaseCodec(valType, signature), elemEncoder: elemEncoder}
		return &singlePointerFix{ValEncoder: encoder}, nil
	}
	return nil, fmt.Errorf("unsupported type %s", valType.String())
}

func createDecoderOfType(cfg *frozenConfig, valType reflect.Type) (ValDecoder, error) {
	valKind := valType.Kind()
	switch valKind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Uintptr, reflect.Float32, reflect.Float64:
		return &NoopCodec{BaseCodec: *NewBaseCodec(valType, uint32(valKind))}, nil
	case reflect.String:
		return &stringCodec{BaseCodec: *NewBaseCodec(valType, uint32(valKind))}, nil
	case reflect.Struct:
		fields := make([]structFieldDecoder, 0, valType.NumField())
		signature := uint32(valKind)
		for i := 0; i < valType.NumField(); i++ {
			decoder, err := createDecoderOfType(cfg, valType.Field(i).Type)
			if err != nil {
				return nil, err
			}
			signature = 31 * signature + decoder.Signature()
			if !decoder.IsNoop() {
				fields = append(fields, structFieldDecoder{
					offset:  valType.Field(i).Offset,
					decoder: decoder,
				})
			}
		}
		return &structDecoder{BaseCodec: *NewBaseCodec(valType, signature), fields: fields}, nil
	case reflect.Slice:
		signature := uint32(valKind)
		elemDecoder, err := createDecoderOfType(cfg, valType.Elem())
		if err != nil {
			return nil, err
		}
		signature = 31 * signature + elemDecoder.Signature()
		if elemDecoder.IsNoop() {
			elemDecoder = nil
		}
		return &sliceDecoder{BaseCodec: *NewBaseCodec(valType, signature),
			elemSize: int(valType.Elem().Size()), elemDecoder: elemDecoder}, nil
	case reflect.Ptr:
		signature := uint32(valKind)
		elemDecoder, err := createDecoderOfType(cfg, valType.Elem())
		if err != nil {
			return nil, err
		}
		signature = 31 * signature + elemDecoder.Signature()
		return &pointerDecoder{BaseCodec: *NewBaseCodec(valType, signature), elemDecoder: elemDecoder}, nil
	}
	return nil, fmt.Errorf("unsupported type %s", valType.String())
}
