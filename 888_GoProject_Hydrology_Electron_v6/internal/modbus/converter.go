package modbus

import (
	"encoding/binary"
	"fmt"
	"math"

	"modbus-tcp-driver-v3/internal/model"
)

func reorderBytes(data []byte, order model.ByteOrder) []byte {
	out := append([]byte(nil), data...)
	if len(out) <= 1 || order == "" || order == model.ABCD {
		return out
	}
	switch order {
	case model.BADC:
		if len(out)%2 != 0 {
			return out
		}
		for i := 0; i < len(out); i += 2 {
			out[i], out[i+1] = out[i+1], out[i]
		}
	case model.CDAB:
		if len(out)%4 != 0 {
			return out
		}
		for i := 0; i < len(out); i += 4 {
			out[i], out[i+2] = out[i+2], out[i]
			out[i+1], out[i+3] = out[i+3], out[i+1]
		}
	case model.DCBA:
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}

func Decode(data []byte, dt model.DataType, order model.ByteOrder) (any, error) {
	data = reorderBytes(data, order)
	required := int(RegisterCount(dt) * 2)
	if dt == model.Bool {
		if len(data) < 1 {
			return nil, fmt.Errorf("bool response is empty")
		}
		return data[0] != 0, nil
	}
	if len(data) < required {
		return nil, fmt.Errorf("not enough bytes: need %d got %d", required, len(data))
	}

	switch dt {
	case model.Int16:
		return int16(binary.BigEndian.Uint16(data[:2])), nil
	case model.UInt16:
		return binary.BigEndian.Uint16(data[:2]), nil
	case model.Int32:
		return int32(binary.BigEndian.Uint32(data[:4])), nil
	case model.UInt32:
		return binary.BigEndian.Uint32(data[:4]), nil
	case model.Float32:
		return math.Float32frombits(binary.BigEndian.Uint32(data[:4])), nil
	case model.Float64:
		return math.Float64frombits(binary.BigEndian.Uint64(data[:8])), nil
	default:
		return nil, fmt.Errorf("unsupported data type: %s", dt)
	}
}

func Encode(value any, dt model.DataType, order model.ByteOrder) ([]byte, error) {
	buf := make([]byte, 8)

	switch dt {
	case model.Int16:
		f, ok := asInt64(value)
		if !ok {
			return nil, fmt.Errorf("cannot convert %T to int16", value)
		}
		binary.BigEndian.PutUint16(buf[:2], uint16(int16(f)))
		return reorderBytes(buf[:2], order), nil
	case model.UInt16:
		f, ok := asFloat64(value)
		if !ok {
			return nil, fmt.Errorf("cannot convert %T to uint16", value)
		}
		binary.BigEndian.PutUint16(buf[:2], uint16(f))
		return reorderBytes(buf[:2], order), nil
	case model.Int32:
		f, ok := asInt64(value)
		if !ok {
			return nil, fmt.Errorf("cannot convert %T to int32", value)
		}
		binary.BigEndian.PutUint32(buf[:4], uint32(int32(f)))
		return reorderBytes(buf[:4], order), nil
	case model.UInt32:
		f, ok := asFloat64(value)
		if !ok {
			return nil, fmt.Errorf("cannot convert %T to uint32", value)
		}
		binary.BigEndian.PutUint32(buf[:4], uint32(f))
		return reorderBytes(buf[:4], order), nil
	case model.Float32:
		f, ok := asFloat64(value)
		if !ok {
			return nil, fmt.Errorf("cannot convert %T to float32", value)
		}
		binary.BigEndian.PutUint32(buf[:4], math.Float32bits(float32(f)))
		return reorderBytes(buf[:4], order), nil
	case model.Float64:
		f, ok := asFloat64(value)
		if !ok {
			return nil, fmt.Errorf("cannot convert %T to float64", value)
		}
		binary.BigEndian.PutUint64(buf[:8], math.Float64bits(f))
		return reorderBytes(buf[:8], order), nil
	default:
		return nil, fmt.Errorf("unsupported data type: %s", dt)
	}
}

func asFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	default:
		return 0, false
	}
}

func asInt64(v any) (int64, bool) {
	f, ok := asFloat64(v)
	return int64(f), ok
}

func Scale(value any, scale, offset float64) any {
	if scale == 0 {
		scale = 1
	}
	f, ok := asFloat64(value)
	if !ok {
		return value
	}
	return f*scale + offset
}

func Unscale(value any, scale, offset float64) (any, error) {
	if scale == 0 {
		scale = 1
	}
	f, ok := asFloat64(value)
	if !ok {
		return nil, fmt.Errorf("value %T is not numeric", value)
	}
	return (f - offset) / scale, nil
}
