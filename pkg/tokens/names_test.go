// Copyright 2016 Marapongo, Inc. All rights reserved.

package tokens

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAsName(t *testing.T) {
	var goodNames = []string{
		"simple",  // all alpha.
		"SiMplE",  // mixed-case alpha.
		"simple0", // alphanumeric.
		"SiMpLe0", // mixed-case alphanumeric.
		"_",       // permit underscore.
		"s1MPl3_", // mixed-case alphanumeric/underscore.
		"_s1MPl3", // ditto.
	}
	for _, nm := range goodNames {
		assert.True(t, IsName(nm), "IsName expected to be true: %v", nm)
		assert.Equal(t, nm, string(AsName(nm)), "AsName expected to echo back: %v", nm)
	}

	var goodQNames = []string{
		"namespace/complex",                   // multi-part name.
		"_naMeSpace0/coMpl3x32",               // multi-part, alphanumeric, etc. name.
		"n_ameSpace3/moRenam3sp4ce/_Complex5", // even more complex parts.
	}
	for _, nm := range goodQNames {
		assert.True(t, IsQName(nm), "IsQName expected to be true: %v", nm)
		assert.False(t, IsName(nm), "IsName expected to be false: %v", nm)
		assert.Equal(t, nm, string(AsQName(nm)), "AsQName expected to echo back: %v", nm)
	}

	var badNames = []string{
		"0_s1MPl3",                         // cannot start with a number.
		"namespace/0complex",               // ditto.
		"namespace/morenamespace/0complex", // ditto.
		"s!mple",                          // bad characters.
		"namesp@ce/complex",               // ditto.
		"namespace/morenamespace/compl#x", // ditto.
	}
	for _, nm := range badNames {
		assert.False(t, IsName(nm), "IsName expected to be false: %v", nm)
		assert.False(t, IsQName(nm), "IsQName expected to be false: %v", nm)
	}
}

func TestNameSimple(t *testing.T) {
	assert.Equal(t, "simple", string(AsName("simple")))
	assert.Equal(t, "complex", string(AsQName("namespace/complex").Name()))
	assert.Equal(t, "complex", string(AsQName("ns1/ns2/ns3/ns4/complex").Name()))
	assert.Equal(t, "c0Mpl3x_", string(AsQName("_/_/_/_/a0/c0Mpl3x_").Name()))
}

func TestNameNamespace(t *testing.T) {
	assert.Equal(t, "namespace", string(AsQName("namespace/complex").Namespace()))
	assert.Equal(t, "ns1/ns2/ns3/ns4", string(AsQName("ns1/ns2/ns3/ns4/complex").Namespace()))
	assert.Equal(t, "_/_/_/_/a0", string(AsQName("_/_/_/_/a0/c0Mpl3x_").Namespace()))
}