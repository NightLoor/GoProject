package modbus

import (
	"fmt"
	"strconv"
	"strings"

	"modbus-tcp-driver-v3/internal/model"
)

type AddressInfo struct {
	Offset       uint16
	RegisterType model.RegisterType
}

func ParseAddress(s string, requested model.RegisterType) (AddressInfo, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return AddressInfo{}, fmt.Errorf("empty modbus address")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return AddressInfo{}, fmt.Errorf("invalid modbus address %q: %w", s, err)
	}

	// Five-digit IEC/Modbus reference style.
	if len(s) >= 5 {
		switch s[0] {
		case '0':
			n--
			if requested != "" && requested != model.Coil {
				return AddressInfo{}, fmt.Errorf("%s implies coil, configured as %s", s, requested)
			}
			return checked(n, model.Coil)
		case '1':
			n -= 10001
			if requested != "" && requested != model.DiscreteInput {
				return AddressInfo{}, fmt.Errorf("%s implies discrete_input, configured as %s", s, requested)
			}
			return checked(n, model.DiscreteInput)
		case '3':
			n -= 30001
			if requested != "" && requested != model.InputRegister {
				return AddressInfo{}, fmt.Errorf("%s implies input_register, configured as %s", s, requested)
			}
			return checked(n, model.InputRegister)
		case '4':
			n -= 40001
			if requested != "" && requested != model.HoldingRegister {
				return AddressInfo{}, fmt.Errorf("%s implies holding_register, configured as %s", s, requested)
			}
			return checked(n, model.HoldingRegister)
		}
	}

	return checked(n, requested)
}

func checked(n int, rt model.RegisterType) (AddressInfo, error) {
	if n < 0 || n > 65535 {
		return AddressInfo{}, fmt.Errorf("modbus offset out of range: %d", n)
	}
	return AddressInfo{Offset: uint16(n), RegisterType: rt}, nil
}

func RegisterCount(dt model.DataType) uint16 {
	switch dt {
	case model.Int32, model.UInt32, model.Float32:
		return 2
	case model.Float64:
		return 4
	default:
		return 1
	}
}
