package main

import (
	"testing"

	"zonebuilder/internal/coverage"
)

func TestWorstPins(t *testing.T) {
	r := coverage.Report{
		Measured:       true,
		GroundMin:      coverage.Spot{X: 1, Y: 2, Z: -96},
		GroundMax:      coverage.Spot{X: 3, Y: 4, Z: 1588},
		FloorClearance: -96.4,
		TopClearance:   1412,
	}
	pins := worstPins(r)
	if len(pins) != 2 {
		t.Fatalf("%d pinos, quero 2", len(pins))
	}
	top, floor := pins[0], pins[1]
	if top.at != r.GroundMax || top.text != "topo +1.412" || top.alert {
		t.Errorf("topo = %+v", top)
	}
	if floor.at != r.GroundMin || floor.text != "piso −96" || !floor.alert {
		t.Errorf("piso = %+v", floor)
	}
	if worstPins(coverage.Report{}) != nil {
		t.Error("sem chão medido não há pinos")
	}
}
