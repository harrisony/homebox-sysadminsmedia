package set

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type args struct {
	a Set[string]
	b Set[string]
}

// Fresh fixtures prevent mutations from leaking through shared `Set` maps.
func argsBasic() args {
	return args{
		a: New("a", "b", "c"),
		b: New("b", "c", "d"),
	}
}

func argsNoOverlap() args {
	return args{
		a: New("a", "b", "c"),
		b: New("d", "e", "f"),
	}
}

func argsIdentical() args {
	return args{
		a: New("a", "b", "c"),
		b: New("a", "b", "c"),
	}
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		args args
		want Set[string]
	}{
		{
			name: "diff basic",
			args: argsBasic(),
			want: New("a"),
		},
		{
			name: "diff empty",
			args: argsIdentical(),
			want: New[string](),
		},
		{
			name: "diff no overlap",
			args: argsNoOverlap(),
			want: New("a", "b", "c"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Diff(tt.args.a, tt.args.b))
		})
	}
}

func TestIntersect(t *testing.T) {
	tests := []struct {
		name string
		args args
		want Set[string]
	}{
		{
			name: "intersect basic",
			args: argsBasic(),
			want: New("b", "c"),
		},
		{
			name: "identical sets",
			args: argsIdentical(),
			want: New("a", "b", "c"),
		},
		{
			name: "no overlap",
			args: argsNoOverlap(),
			want: New[string](),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Intersect(tt.args.a, tt.args.b))
		})
	}
}

func TestUnion(t *testing.T) {
	tests := []struct {
		name string
		args args
		want Set[string]
	}{
		{
			name: "intersect basic",
			args: argsBasic(),
			want: New("a", "b", "c", "d"),
		},
		{
			name: "identical sets",
			args: argsIdentical(),
			want: New("a", "b", "c"),
		},
		{
			name: "no overlap",
			args: argsNoOverlap(),
			want: New("a", "b", "c", "d", "e", "f"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Union(tt.args.a, tt.args.b))
		})
	}
}

func TestXor(t *testing.T) {
	tests := []struct {
		name string
		args args
		want Set[string]
	}{
		{
			name: "xor basic",
			args: argsBasic(),
			want: New("a", "d"),
		},
		{
			name: "identical sets",
			args: argsIdentical(),
			want: New[string](),
		},
		{
			name: "no overlap",
			args: argsNoOverlap(),
			want: New("a", "b", "c", "d", "e", "f"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Xor(tt.args.a, tt.args.b))
		})
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "equal basic",
			args: argsBasic(),
			want: false,
		},
		{
			name: "identical sets",
			args: argsIdentical(),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Equal(tt.args.a, tt.args.b))
		})
	}
}

func TestSubset(t *testing.T) {
	type args struct {
		a Set[string]
		b Set[string]
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "subset basic",
			args: args{
				a: New("a", "b"),
				b: New("a", "b", "c"),
			},
			want: true,
		},
		{
			name: "subset basic false",
			args: args{
				a: New("a", "b", "d"),
				b: New("a", "b", "c"),
			},
			want: false,
		},
		{
			name: "equal non-empty sets are subsets of each other",
			args: args{
				a: New("a", "b", "c"),
				b: New("a", "b", "c"),
			},
			want: true,
		},
		{
			name: "equal empty sets are subsets of each other",
			args: args{
				a: New[string](),
				b: New[string](),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Subset(tt.args.a, tt.args.b))
		})
	}
}

func TestSuperset(t *testing.T) {
	type args struct {
		a Set[string]
		b Set[string]
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "superset basic",
			args: args{
				a: New("a", "b", "c"),
				b: New("a", "b"),
			},
			want: true,
		},
		{
			name: "superset basic false",
			args: args{
				a: New("a", "b", "c"),
				b: New("a", "b", "d"),
			},
			want: false,
		},
		{
			name: "equal non-empty sets are supersets of each other",
			args: args{
				a: New("a", "b", "c"),
				b: New("a", "b", "c"),
			},
			want: true,
		},
		{
			name: "equal empty sets are supersets of each other",
			args: args{
				a: New[string](),
				b: New[string](),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Superset(tt.args.a, tt.args.b))
		})
	}
}

func TestDisjoint(t *testing.T) {
	type args struct {
		a Set[string]
		b Set[string]
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "disjoint basic",
			args: args{
				a: New("a", "b"),
				b: New("c", "d"),
			},
			want: true,
		},
		{
			name: "disjoint basic false",
			args: args{
				a: New("a", "b"),
				b: New("b", "c"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Disjoint(tt.args.a, tt.args.b))
		})
	}
}
