package modbus

import (
	"fmt"
	"sort"

	"modbus-tcp-driver-v3/internal/model"
)

type Tag struct {
	Config       model.TagConfig
	Offset       uint16
	RegisterType model.RegisterType
	Width        uint16
}

func (t Tag) End() uint16 {
	return t.Offset + t.Width - 1
}

type Batch struct {
	SlaveID      byte
	RegisterType model.RegisterType
	Start        uint16
	Quantity     uint16
	Tags         []Tag
}

func BuildBatches(tags []Tag, batchGap, maxQuantity uint16) ([]Batch, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	if maxQuantity == 0 {
		return nil, fmt.Errorf("max batch quantity is zero")
	}

	cp := append([]Tag(nil), tags...)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].Offset < cp[j].Offset
	})

	var batches []Batch
	current := Batch{
		SlaveID:      cp[0].Config.SlaveID,
		RegisterType: cp[0].RegisterType,
		Start:        cp[0].Offset,
		Quantity:     cp[0].Width,
		Tags:         []Tag{cp[0]},
	}
	currentEnd := cp[0].End()

	flush := func() {
		batches = append(batches, current)
	}

	for i := 1; i < len(cp); i++ {
		t := cp[i]
		if t.Config.SlaveID != current.SlaveID || t.RegisterType != current.RegisterType {
			flush()
			current = Batch{SlaveID: t.Config.SlaveID, RegisterType: t.RegisterType, Start: t.Offset, Quantity: t.Width, Tags: []Tag{t}}
			currentEnd = t.End()
			continue
		}

		if t.Offset <= currentEnd+1 {
			if t.End()-current.Start+1 <= maxQuantity {
				current.Tags = append(current.Tags, t)
				currentEnd = maxUint16(currentEnd, t.End())
				current.Quantity = currentEnd - current.Start + 1
				continue
			}
		} else {
			gap := t.Offset - currentEnd - 1
			combined := t.End() - current.Start + 1
			if gap <= batchGap && combined <= maxQuantity {
				current.Tags = append(current.Tags, t)
				currentEnd = t.End()
				current.Quantity = combined
				continue
			}
		}

		flush()
		current = Batch{SlaveID: t.Config.SlaveID, RegisterType: t.RegisterType, Start: t.Offset, Quantity: t.Width, Tags: []Tag{t}}
		currentEnd = t.End()
	}

	flush()
	return batches, nil
}

func maxUint16(a, b uint16) uint16 {
	if a > b {
		return a
	}
	return b
}
