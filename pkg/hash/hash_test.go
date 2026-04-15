package hash

import (
	"testing"

	"github.com/DaviRain-Su/infracast/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// Test 1: Same input produces same hash
func TestSpecHash_Determinism(t *testing.T) {
	spec := providers.DatabaseSpec{
		Engine:        "mysql",
		Version:       "8.0",
		InstanceClass: "rds.mysql.s1.small",
		StorageGB:     50,
		HighAvail:     boolPtr(true),
	}

	hash1, err := SpecHash(ResourceTypeDatabase, spec)
	require.NoError(t, err)

	hash2, err := SpecHash(ResourceTypeDatabase, spec)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "same input should produce same hash")
}

// Test 2: Different specs produce different hashes
func TestSpecHash_DifferentSpecs(t *testing.T) {
	spec1 := providers.DatabaseSpec{
		Engine:    "mysql",
		Version:   "8.0",
		StorageGB: 50,
	}
	spec2 := providers.DatabaseSpec{
		Engine:    "mysql",
		Version:   "5.7",
		StorageGB: 50,
	}

	hash1, err := SpecHash(ResourceTypeDatabase, spec1)
	require.NoError(t, err)

	hash2, err := SpecHash(ResourceTypeDatabase, spec2)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "different specs should produce different hashes")
}

// Test 3: Database field coverage
func TestSpecHash_Database_Fields(t *testing.T) {
	tests := []struct {
		name     string
		spec1    providers.DatabaseSpec
		spec2    providers.DatabaseSpec
		sameHash bool
	}{
		{
			name: "different engine",
			spec1: providers.DatabaseSpec{
				Engine:  "mysql",
				Version: "8.0",
			},
			spec2: providers.DatabaseSpec{
				Engine:  "postgresql",
				Version: "8.0",
			},
			sameHash: false,
		},
		{
			name: "different storage",
			spec1: providers.DatabaseSpec{
				Engine:    "mysql",
				StorageGB: 50,
			},
			spec2: providers.DatabaseSpec{
				Engine:    "mysql",
				StorageGB: 100,
			},
			sameHash: false,
		},
		{
			name: "same specs",
			spec1: providers.DatabaseSpec{
				Engine:        "mysql",
				Version:       "8.0",
				InstanceClass: "rds.mysql.s1.small",
				StorageGB:     50,
				HighAvail:     boolPtr(true),
			},
			spec2: providers.DatabaseSpec{
				Engine:        "mysql",
				Version:       "8.0",
				InstanceClass: "rds.mysql.s1.small",
				StorageGB:     50,
				HighAvail:     boolPtr(true),
			},
			sameHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1, err := SpecHash(ResourceTypeDatabase, tt.spec1)
			require.NoError(t, err)

			hash2, err := SpecHash(ResourceTypeDatabase, tt.spec2)
			require.NoError(t, err)

			if tt.sameHash {
				assert.Equal(t, hash1, hash2)
			} else {
				assert.NotEqual(t, hash1, hash2)
			}
		})
	}
}

// Test 4: Metadata exclusion - name should not affect hash
func TestSpecHash_MetadataExclusion(t *testing.T) {
	spec1 := providers.DatabaseSpec{
		Name:      "db1",
		Engine:    "mysql",
		Version:   "8.0",
		StorageGB: 50,
	}
	spec2 := providers.DatabaseSpec{
		Name:      "db2",
		Engine:    "mysql",
		Version:   "8.0",
		StorageGB: 50,
	}

	hash1, err := SpecHash(ResourceTypeDatabase, spec1)
	require.NoError(t, err)

	hash2, err := SpecHash(ResourceTypeDatabase, spec2)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "name should not affect hash (metadata exclusion)")
}

// Test 5: Empty spec handling
func TestSpecHash_EmptySpec(t *testing.T) {
	spec := providers.DatabaseSpec{}

	hash, err := SpecHash(ResourceTypeDatabase, spec)
	require.NoError(t, err)
	assert.NotEmpty(t, hash, "empty spec should still produce a hash")
}

// Test 6: Cache spec hashing
func TestSpecHash_Cache(t *testing.T) {
	spec := providers.CacheSpec{
		Name:     "mycache",
		Engine:   "redis",
		MemoryMB: 1024,
	}

	hash, err := SpecHash(ResourceTypeCache, spec)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

// Test 7: ObjectStorage spec hashing
func TestSpecHash_ObjectStorage(t *testing.T) {
	spec := providers.ObjectStorageSpec{
		Name: "mybucket",
		ACL:  "private",
	}

	hash, err := SpecHash(ResourceTypeObjectStorage, spec)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

// Test 8: CORS rules sorting for determinism
func TestSpecHash_ObjectStorage_CORSSorted(t *testing.T) {
	spec := providers.ObjectStorageSpec{
		Name: "mybucket",
		ACL:  "private",
		CORSRules: []providers.CORSRule{
			{
				AllowedOrigins: []string{"https://b.com", "https://a.com"},
				AllowedMethods: []string{"POST", "GET"},
				AllowedHeaders: []string{"X-B", "X-A"},
			},
		},
	}

	// Multiple calls should produce same hash despite unsorted input
	hash1, err := SpecHash(ResourceTypeObjectStorage, spec)
	require.NoError(t, err)

	hash2, err := SpecHash(ResourceTypeObjectStorage, spec)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "CORS rules should be sorted for determinism")
}

// Test 9: Compute spec with env_vars determinism
func TestSpecHash_Compute_EnvVars(t *testing.T) {
	spec := providers.ComputeSpec{
		ServiceName: "api",
		Replicas:    3,
		CPU:         "1000m",
		Memory:      "512Mi",
		Port:        8080,
		EnvVars: map[string]string{
			"B_KEY": "value2",
			"A_KEY": "value1",
		},
	}

	// Multiple calls should produce same hash despite map iteration order
	hash1, err := SpecHash(ResourceTypeCompute, spec)
	require.NoError(t, err)

	hash2, err := SpecHash(ResourceTypeCompute, spec)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "env_vars should be sorted for determinism")
}

// Test 10: Compute spec env_vars order affects hash
func TestSpecHash_Compute_EnvVars_DifferentValues(t *testing.T) {
	spec1 := providers.ComputeSpec{
		ServiceName: "api",
		EnvVars: map[string]string{
			"KEY": "value1",
		},
	}
	spec2 := providers.ComputeSpec{
		ServiceName: "api",
		EnvVars: map[string]string{
			"KEY": "value2",
		},
	}

	hash1, err := SpecHash(ResourceTypeCompute, spec1)
	require.NoError(t, err)

	hash2, err := SpecHash(ResourceTypeCompute, spec2)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "different env var values should produce different hashes")
}

// Test 11: Unknown resource type returns error
func TestSpecHash_UnknownType(t *testing.T) {
	_, err := SpecHash("unknown_type", providers.DatabaseSpec{})
	assert.Error(t, err)
	assert.Equal(t, ErrUnknownResourceType, err)
}

// Test 12: Compute with secret_refs
func TestSpecHash_Compute_SecretRefs(t *testing.T) {
	spec := providers.ComputeSpec{
		ServiceName: "api",
		SecretRefs:  []string{"secret-b", "secret-a"},
	}

	hash, err := SpecHash(ResourceTypeCompute, spec)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

// Test 13: Wrong type returns error
func TestSpecHash_WrongType(t *testing.T) {
	_, err := SpecHash(ResourceTypeDatabase, providers.CacheSpec{})
	assert.Error(t, err)
}

// Test 14: SpecHashResource wrapper
func TestSpecHashResource(t *testing.T) {
	spec := providers.ResourceSpec{
		Type: "database",
		DatabaseSpec: &providers.DatabaseSpec{
			Engine:  "mysql",
			Version: "8.0",
		},
	}

	hash, err := SpecHashResource(spec)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

// Test 15: ResourceSpec with nil inner spec returns error
func TestSpecHashResource_NilSpec(t *testing.T) {
	spec := providers.ResourceSpec{
		Type:         "database",
		DatabaseSpec: nil,
	}

	_, err := SpecHashResource(spec)
	assert.Error(t, err)
}
