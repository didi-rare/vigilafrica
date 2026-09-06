package normalizer

import "testing"

func TestValidLonLat(t *testing.T) {
	valid := [][2]float64{
		{3.3942, 6.4551},   // Lagos — the one flood EONET gets right
		{9.414, 4.6027},    // the Cameroon flood, correct order
		{-79.5, 43.8},      // Toronto, negative longitude
		{180, 90}, {-180, -90}, {0, 0},
	}
	for _, c := range valid {
		if !validLonLat(c[0], c[1]) {
			t.Errorf("expected [%v %v] to be valid", c[0], c[1])
		}
	}

	invalid := [][2]float64{
		{4.6027, 9.414},   // ...still valid numerically; see the note below
		{36.5, 136.9},     // Japan transposed — latitude 136.9 is impossible
		{43.8, -79.5},     // Toronto transposed — latitude -79.5 is valid, longitude 43.8 is too
		{0, 91}, {0, -91}, // latitude out of range
		{181, 0}, {-181, 0},
	}
	// ⚠️ Only the genuinely impossible ones must fail. This test deliberately
	// documents the LIMIT of the guard: {4.6027, 9.414} and {43.8, -79.5} are
	// transposed but numerically valid, so no bounds check can catch them. That
	// gap is why the ingestor resolves geometry from GDACS instead of relying on
	// this function.
	mustFail := map[int]bool{1: true, 3: true, 4: true, 5: true, 6: true}
	for i, c := range invalid {
		got := validLonLat(c[0], c[1])
		if mustFail[i] && got {
			t.Errorf("case %d [%v %v]: expected invalid", i, c[0], c[1])
		}
		if !mustFail[i] && !got {
			t.Errorf("case %d [%v %v]: expected valid (a bounds check cannot detect this transposition)", i, c[0], c[1])
		}
	}
}

func TestPolygonCoordinatesPlausible(t *testing.T) {
	// A well-formed ring in [lon, lat] order.
	good := []interface{}{
		[]interface{}{
			[]interface{}{4.377, 9.216},
			[]interface{}{4.828, 9.216},
			[]interface{}{4.828, 9.569},
			[]interface{}{4.377, 9.216},
		},
	}
	if !polygonCoordinatesPlausible(good) {
		t.Error("a valid ring must be accepted")
	}

	// The real EONET shape for "Flood in Japan 1104140": the second value is
	// 136.77..137.76, which cannot be a latitude.
	japan := []interface{}{
		[]interface{}{
			[]interface{}{36.27, 136.77},
			[]interface{}{37.25, 137.76},
			[]interface{}{36.27, 136.77},
		},
	}
	if polygonCoordinatesPlausible(japan) {
		t.Error("a latitude of 136.77 must be rejected")
	}

	t.Run("every vertex is checked, not just the first", func(t *testing.T) {
		// A partially corrupted ring is harder to spot than a uniformly
		// transposed one and must not pass on a valid opening vertex.
		partial := []interface{}{
			[]interface{}{
				[]interface{}{4.377, 9.216}, // fine
				[]interface{}{4.828, 9.569}, // fine
				[]interface{}{4.828, 168.12}, // impossible
			},
		}
		if polygonCoordinatesPlausible(partial) {
			t.Error("one impossible vertex must reject the whole polygon")
		}
	})

	t.Run("handles MultiPolygon-style nesting", func(t *testing.T) {
		nested := []interface{}{
			[]interface{}{
				[]interface{}{
					[]interface{}{1.0, 2.0},
					[]interface{}{3.0, 200.0}, // impossible, two levels down
				},
			},
		}
		if polygonCoordinatesPlausible(nested) {
			t.Error("nested rings must be walked")
		}
	})

	t.Run("malformed input is rejected rather than panicking", func(t *testing.T) {
		for _, bad := range [][]interface{}{
			{[]interface{}{[]interface{}{1.0}}},                    // single element
			{[]interface{}{[]interface{}{1.0, "x"}}},               // non-numeric
		} {
			if polygonCoordinatesPlausible(bad) {
				t.Errorf("expected %v to be rejected", bad)
			}
		}
		// Empty input has nothing impossible in it; accepting it is correct, and
		// the caller already skips events with no geometry.
		if !polygonCoordinatesPlausible([]interface{}{}) {
			t.Error("empty coordinates should not be treated as implausible")
		}
	})
}
