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

func (cfg *frozenConfig) addEncoderToCache(cacheKey reflect.Type, encoder RootEncoder) {
	done := false
	for !done {
		ptr := atomic.LoadPointer(&cfg.encoderCache)
		cache := *(*map[reflect.Type]RootEncoder)(ptr)
		copied := map[reflect.Type]RootEncoder{}
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

func (cfg *frozenConfig) getEncoderFromCache(cacheKey reflect.Type) RootEncoder {
	ptr := atomic.LoadPointer(&cfg.encoderCache)
	cache := *(*map[reflect.Type]RootEncoder)(ptr)
	return cache[cacheKey]
}

func encoderOfType(cfg *frozenConfig, valType reflect.Type) (RootEncoder, error) {
	cacheKey := valType
	rootEncoder := cfg.getEncoderFromCache(cacheKey)
	if rootEncoder != nil {
		return rootEncoder, nil
	}
	encoder, err := createEncoderOfType(cfg, valType)
	if err != nil {
		return nil, err
	}
	rootEncoder = wrapRootEncoder(encoder)
	cfg.addEncoderToCache(cacheKey, rootEncoder)
	return rootEncoder, err
}

func wrapRootEncoder(encoder ValEncoder) RootEncoder {
	valType := encoder.Type()
	valKind := valType.Kind()
	rootEncoder := rootEncoder{valType, encoder.Signature(), encoder}
	switch valKind {
	case reflect.Struct:
		if valType.NumField() == 1 && valType.Field(0).Type.Kind() == reflect.Ptr {
			return &singlePointerFix{rootEncoder}
		}
	case reflect.Array:
		if valType.Len() == 1 && valType.Elem().Kind() == reflect.Ptr {
			return &singlePointerFix{rootEncoder}
		}
	case reflect.Ptr:
		return &singlePointerFix{rootEncoder}
	}
	return &rootEncoder
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
		return &NoopCodec{BaseCodec: *newBaseCodec(valType, uint32(valKind))}, nil
	case reflect.String:
		return &stringCodec{BaseCodec: *newBaseCodec(valType, uint32(valKind))}, nil
	case reflect.Struct:
		signature := uint32(valKind)
		fields := make([]structFieldEncoder, 0, valType.NumField())
		for i := 0; i < valType.NumField(); i++ {
			encoder, err := createEncoderOfType(cfg, valType.Field(i).Type)
			if err != nil {
				return nil, err
			}
			signature = 31*signature + encoder.Signature()
			if !encoder.IsNoop() {
				fields = append(fields, structFieldEncoder{
					offset:  valType.Field(i).Offset,
					encoder: encoder,
				})
			}
		}
		encoder := &structEncoder{BaseCodec: *newBaseCodec(valType, signature), fields: fields}
		return encoder, nil
	case reflect.Array:
		signature := uint32(valKind)
		elemEncoder, err := createEncoderOfType(cfg, valType.Elem())
		if err != nil {
			return nil, err
		}
		signature = 31*signature + elemEncoder.Signature()
		if elemEncoder.IsNoop() {
			elemEncoder = nil
		}
		encoder := &arrayEncoder{
			BaseCodec:   *newBaseCodec(valType, signature),
			arrayLength: valType.Len(),
			elementSize: valType.Elem().Size(),
			elemEncoder: elemEncoder,
		}
		return encoder, nil
	case reflect.Slice:
		signature := uint32(valKind)
		elemEncoder, err := createEncoderOfType(cfg, valType.Elem())
		if err != nil {
			return nil, err
		}
		signature = 31*signature + elemEncoder.Signature()
		if elemEncoder.IsNoop() {
			elemEncoder = nil
		}
		return &sliceEncoder{BaseCodec: *newBaseCodec(valType, signature),
			elemSize: int(valType.Elem().Size()), elemEncoder: elemEncoder}, nil
	case reflect.Ptr:
		signature := uint32(valKind)
		elemEncoder, err := createEncoderOfType(cfg, valType.Elem())
		if err != nil {
			return nil, err
		}
		signature = 31*signature + elemEncoder.Signature()
		encoder := &pointerEncoder{BaseCodec: *newBaseCodec(valType, signature), elemEncoder: elemEncoder}
		return encoder, nil
	}
	return nil, fmt.Errorf("unsupported type %s", valType.String())
}

func createDecoderOfType(cfg *frozenConfig, valType reflect.Type) (ValDecoder, error) {
	valKind := valType.Kind()
	switch valKind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Uintptr, reflect.Float32, reflect.Float64:
		return &NoopCodec{BaseCodec: *newBaseCodec(valType, uint32(valKind))}, nil
	case reflect.String:
		return &stringCodec{BaseCodec: *newBaseCodec(valType, uint32(valKind))}, nil
	case reflect.Struct:
		fields := make([]structFieldDecoder, 0, valType.NumField())
		signature := uint32(valKind)
		hasPointer := false
		for i := 0; i < valType.NumField(); i++ {
			decoder, err := createDecoderOfType(cfg, valType.Field(i).Type)
			if err != nil {
				return nil, err
			}
			if decoder.HasPointer() {
				hasPointer = true
			}
			signature = 31*signature + decoder.Signature()
			if !decoder.IsNoop() {
				fields = append(fields, structFieldDecoder{
					offset:  valType.Field(i).Offset,
					decoder: decoder,
				})
			}
		}
		if hasPointer {
			return &structDecoderWithPointer{BaseCodec: *newBaseCodec(valType, signature), fields: fields}, nil
		}
		return &structDecoderWithoutPointer{BaseCodec: *newBaseCodec(valType, signature), fields: fields}, nil
	case reflect.Array:
		signature := uint32(valKind)
		elemDecoder, err := createDecoderOfType(cfg, valType.Elem())
		if err != nil {
			return nil, err
		}
		signature = 31*signature + elemDecoder.Signature()
		hasPointer := elemDecoder.HasPointer()
		if elemDecoder.IsNoop() {
			elemDecoder = nil
		}
		if hasPointer {
			return &arrayDecoderWithPointer{
				BaseCodec:   *newBaseCodec(valType, signature),
				arrayLength: valType.Len(),
				elementSize: valType.Elem().Size(),
				elemDecoder: elemDecoder,
			}, nil
		}
		return &arrayDecoderWithoutPointer{
			BaseCodec:   *newBaseCodec(valType, signature),
			arrayLength: valType.Len(),
			elementSize: valType.Elem().Size(),
			elemDecoder: elemDecoder,
		}, nil
	case reflect.Slice:
		signature := uint32(valKind)
		elemDecoder, err := createDecoderOfType(cfg, valType.Elem())
		if err != nil {
			return nil, err
		}
		signature = 31*signature + elemDecoder.Signature()
		shouldCopy := false
		if elemDecoder.HasPointer() && cfg.readonlyDecode {
			shouldCopy = true
		}
		if elemDecoder.IsNoop() {
			elemDecoder = nil
		}
		if shouldCopy {
			return &sliceDecoderWithCopy{BaseCodec: *newBaseCodec(valType, signature),
				elemSize: int(valType.Elem().Size()), elemDecoder: elemDecoder}, nil
		}
		return &sliceDecoderWithoutCopy{BaseCodec: *newBaseCodec(valType, signature),
			elemSize: int(valType.Elem().Size()), elemDecoder: elemDecoder}, nil
	case reflect.Ptr:
		signature := uint32(valKind)
		elemDecoder, err := createDecoderOfType(cfg, valType.Elem())
		if err != nil {
			return nil, err
		}
		signature = 31*signature + elemDecoder.Signature()
		if elemDecoder.HasPointer() && cfg.readonlyDecode {
			return &pointerDecoderWithCopy{BaseCodec: *newBaseCodec(valType, signature), elemDecoder: elemDecoder}, nil
		}
		return &pointerDecoderWithoutCopy{BaseCodec: *newBaseCodec(valType, signature), elemDecoder: elemDecoder}, nil
	}
	return nil, fmt.Errorf("unsupported type %s", valType.String())
}
