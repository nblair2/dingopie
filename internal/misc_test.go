package internal_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nblair2/dingopie/internal"
)

func TestGetPointVarianceRange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		points    int
		variance  float32
		maxPoints int
		wantLow   int
		wantHigh  int
	}{
		{
			name:      "negative variance returns points unchanged",
			points:    100,
			variance:  -0.1,
			maxPoints: 1000,
			wantLow:   100,
			wantHigh:  100,
		},
		{
			name:      "zero variance returns points unchanged",
			points:    100,
			variance:  0,
			maxPoints: 1000,
			wantLow:   100,
			wantHigh:  100,
		},
		{
			name:      "variance equal to one uses max(2*points, maxPoints)",
			points:    100,
			variance:  1,
			maxPoints: 1000,
			wantLow:   100,
			wantHigh:  1000,
		},
		{
			name:      "variance greater than one with small maxPoints uses 2*points",
			points:    100,
			variance:  1.5,
			maxPoints: 50,
			wantLow:   100,
			wantHigh:  200,
		},
		{
			name:      "mid variance computes symmetric range",
			points:    100,
			variance:  0.5,
			maxPoints: 1000,
			wantLow:   50,
			wantHigh:  150,
		},
		{
			name:      "high end clamps to maxPoints",
			points:    100,
			variance:  0.9,
			maxPoints: 150,
			wantLow:   10,
			wantHigh:  150,
		},
		{
			name:      "low end clamps to one",
			points:    1,
			variance:  0.5,
			maxPoints: 100,
			wantLow:   1,
			wantHigh:  1,
		},
		{
			name:      "zero points",
			points:    0,
			variance:  0.5,
			maxPoints: 100,
			wantLow:   1,
			wantHigh:  0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotLow, gotHigh := internal.GetPointVarianceRange(
				tc.points, tc.variance, tc.maxPoints,
			)
			if gotLow != tc.wantLow || gotHigh != tc.wantHigh {
				t.Errorf(
					"GetPointVarianceRange(%d, %v, %d) = (%d, %d), want (%d, %d)",
					tc.points, tc.variance, tc.maxPoints,
					gotLow, gotHigh, tc.wantLow, tc.wantHigh,
				)
			}
		})
	}
}

func TestInsertPeriodicBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		source    []byte
		insertion []byte
		offset    int
		period    int
		want      []byte
		wantErr   string
	}{
		{
			name:      "godoc example for insert",
			source:    []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6},
			insertion: []byte{0xA, 0xB},
			offset:    2,
			period:    2,
			want: []byte{
				0x1, 0x2,
				0xA, 0xB, 0x3, 0x4,
				0xA, 0xB, 0x5, 0x6,
				0xA, 0xB,
			},
		},
		{
			name:      "zero offset",
			source:    []byte{0x1, 0x2, 0x3, 0x4},
			insertion: []byte{0xFF},
			offset:    0,
			period:    2,
			want: []byte{
				0xFF, 0x1, 0x2,
				0xFF, 0x3, 0x4,
				0xFF,
			},
		},
		{
			name:      "offset equal to source length appends only trailing insertion",
			source:    []byte{0x1, 0x2, 0x3, 0x4},
			insertion: []byte{0xAA},
			offset:    4,
			period:    2,
			want:      []byte{0x1, 0x2, 0x3, 0x4, 0xAA},
		},
		{
			name:      "period not dividing remaining source returns error",
			source:    []byte{0x1, 0x2, 0x3, 0x4, 0x5},
			insertion: []byte{0xA, 0xB},
			offset:    2,
			period:    2,
			wantErr:   "multiple of period",
		},
		{
			name:      "empty insertion is a no-op",
			source:    []byte{0x1, 0x2, 0x3, 0x4},
			insertion: []byte{},
			offset:    0,
			period:    2,
			want:      []byte{0x1, 0x2, 0x3, 0x4},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := internal.InsertPeriodicBytes(
				tc.source, tc.insertion, tc.offset, tc.period,
			)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}

				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("InsertPeriodicBytes mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRemovePeriodicBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		source    []byte
		insertLen int
		offset    int
		period    int
		want      []byte
		wantErr   string
	}{
		{
			name: "godoc example for remove",
			source: []byte{
				0x1, 0x2,
				0xA, 0xB, 0x3, 0x4,
				0xA, 0xB, 0x5, 0x6,
				0xA, 0xB,
			},
			insertLen: 2,
			offset:    2,
			period:    2,
			want:      []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6},
		},
		{
			name:      "zero offset",
			source:    []byte{0xFF, 0x1, 0x2, 0xFF, 0x3, 0x4, 0xFF},
			insertLen: 1,
			offset:    0,
			period:    2,
			want:      []byte{0x1, 0x2, 0x3, 0x4},
		},
		{
			name:      "offset greater than source length returns error",
			source:    []byte{0x1, 0x2},
			insertLen: 1,
			offset:    5,
			period:    2,
			wantErr:   "is larger than source length",
		},
		{
			name:      "incomplete trailing insertion returns error",
			source:    []byte{0x1, 0x2, 0xA},
			insertLen: 2,
			offset:    2,
			period:    2,
			wantErr:   "incomplete insertion sequence",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := internal.RemovePeriodicBytes(
				tc.source, tc.insertLen, tc.offset, tc.period,
			)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}

				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("RemovePeriodicBytes mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPeriodicBytesRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		source    []byte
		insertion []byte
		offset    int
		period    int
	}{
		{
			name:      "godoc example",
			source:    []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6},
			insertion: []byte{0xA, 0xB},
			offset:    2,
			period:    2,
		},
		{
			name:      "zero offset, single byte insertion",
			source:    []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x12, 0x34, 0x56, 0x78},
			insertion: []byte{0xFF},
			offset:    0,
			period:    4,
		},
		{
			name:      "large period",
			source:    []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xA},
			insertion: []byte{0xCC, 0xDD, 0xEE},
			offset:    0,
			period:    5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			inserted, err := internal.InsertPeriodicBytes(
				tc.source, tc.insertion, tc.offset, tc.period,
			)
			if err != nil {
				t.Fatalf("InsertPeriodicBytes: %v", err)
			}

			restored, err := internal.RemovePeriodicBytes(
				inserted, len(tc.insertion), tc.offset, tc.period,
			)
			if err != nil {
				t.Fatalf("RemovePeriodicBytes: %v", err)
			}

			if diff := cmp.Diff(tc.source, restored); diff != "" {
				t.Errorf("round trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPadDataToChunkSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		data      []byte
		chunkSize int
		wantLen   int
	}{
		{
			name:      "already aligned returns unchanged length",
			data:      []byte{0x1, 0x2, 0x3, 0x4},
			chunkSize: 4,
			wantLen:   4,
		},
		{
			name:      "one byte short pads to boundary",
			data:      []byte{0x1, 0x2, 0x3},
			chunkSize: 4,
			wantLen:   4,
		},
		{
			name:      "one byte over pads to next boundary",
			data:      []byte{0x1, 0x2, 0x3, 0x4, 0x5},
			chunkSize: 4,
			wantLen:   8,
		},
		{
			name:      "empty input is already aligned",
			data:      []byte{},
			chunkSize: 4,
			wantLen:   0,
		},
		{
			name:      "non-power-of-two chunk size",
			data:      []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7},
			chunkSize: 5,
			wantLen:   10,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			original := append([]byte(nil), tc.data...)
			got := internal.PadDataToChunkSize(tc.data, tc.chunkSize)

			if len(got) != tc.wantLen {
				t.Errorf("len = %d, want %d", len(got), tc.wantLen)
			}

			if tc.chunkSize > 0 && len(got)%tc.chunkSize != 0 {
				t.Errorf("len %d is not a multiple of chunkSize %d", len(got), tc.chunkSize)
			}

			if !bytes.Equal(original, got[:len(original)]) {
				t.Errorf(
					"leading bytes were modified: got %#v, want %#v",
					got[:len(original)],
					original,
				)
			}
		})
	}
}
