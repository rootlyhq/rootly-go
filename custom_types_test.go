package rootly

import (
	"encoding/json"
	"testing"
)

func TestID_Basic(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "UUID format",
			value: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:  "slug format",
			value: "my-incident-slug",
		},
		{
			name:  "slug with underscores",
			value: "my_incident_slug",
		},
		{
			name:  "numeric string",
			value: "12345",
		},
		{
			name:  "empty string",
			value: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := ID(tt.value)
			if string(id) != tt.value {
				t.Errorf("ID value = %v, want %v", string(id), tt.value)
			}
		})
	}
}

func TestID_JSONMarshal(t *testing.T) {
	tests := []struct {
		name     string
		id       ID
		expected string
	}{
		{
			name:     "UUID",
			id:       ID("550e8400-e29b-41d4-a716-446655440000"),
			expected: `"550e8400-e29b-41d4-a716-446655440000"`,
		},
		{
			name:     "slug",
			id:       ID("my-incident"),
			expected: `"my-incident"`,
		},
		{
			name:     "empty",
			id:       ID(""),
			expected: `""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.id)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("json.Marshal() = %v, want %v", string(data), tt.expected)
			}
		})
	}
}

func TestID_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected ID
		wantErr  bool
	}{
		{
			name:     "UUID",
			json:     `"550e8400-e29b-41d4-a716-446655440000"`,
			expected: ID("550e8400-e29b-41d4-a716-446655440000"),
			wantErr:  false,
		},
		{
			name:     "slug",
			json:     `"my-incident"`,
			expected: ID("my-incident"),
			wantErr:  false,
		},
		{
			name:     "empty string",
			json:     `""`,
			expected: ID(""),
			wantErr:  false,
		},
		{
			name:     "null",
			json:     `null`,
			expected: ID(""),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var id ID
			err := json.Unmarshal([]byte(tt.json), &id)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if id != tt.expected {
				t.Errorf("json.Unmarshal() = %v, want %v", id, tt.expected)
			}
		})
	}
}

func TestID_InStruct(t *testing.T) {
	type TestStruct struct {
		ID   ID     `json:"id"`
		Name string `json:"name"`
	}

	t.Run("marshal struct with UUID", func(t *testing.T) {
		s := TestStruct{
			ID:   ID("550e8400-e29b-41d4-a716-446655440000"),
			Name: "test",
		}
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		expected := `{"id":"550e8400-e29b-41d4-a716-446655440000","name":"test"}`
		if string(data) != expected {
			t.Errorf("json.Marshal() = %v, want %v", string(data), expected)
		}
	})

	t.Run("unmarshal struct with slug", func(t *testing.T) {
		jsonData := `{"id":"my-incident","name":"test"}`
		var s TestStruct
		err := json.Unmarshal([]byte(jsonData), &s)
		if err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if s.ID != ID("my-incident") {
			t.Errorf("ID = %v, want %v", s.ID, ID("my-incident"))
		}
		if s.Name != "test" {
			t.Errorf("Name = %v, want %v", s.Name, "test")
		}
	})

	t.Run("round trip", func(t *testing.T) {
		original := TestStruct{
			ID:   ID("my-slug-123"),
			Name: "test incident",
		}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		var decoded TestStruct
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if decoded.ID != original.ID {
			t.Errorf("ID = %v, want %v", decoded.ID, original.ID)
		}
		if decoded.Name != original.Name {
			t.Errorf("Name = %v, want %v", decoded.Name, original.Name)
		}
	})
}

func TestID_Comparison(t *testing.T) {
	id1 := ID("550e8400-e29b-41d4-a716-446655440000")
	id2 := ID("550e8400-e29b-41d4-a716-446655440000")
	id3 := ID("my-incident")

	if id1 != id2 {
		t.Errorf("identical IDs should be equal")
	}

	if id1 == id3 {
		t.Errorf("different IDs should not be equal")
	}
}

func TestID_String(t *testing.T) {
	tests := []struct {
		name     string
		id       ID
		expected string
	}{
		{
			name:     "UUID",
			id:       ID("550e8400-e29b-41d4-a716-446655440000"),
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "slug",
			id:       ID("my-incident"),
			expected: "my-incident",
		},
		{
			name:     "empty",
			id:       ID(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.id.String()
			if result != tt.expected {
				t.Errorf("ID.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestID_UUID(t *testing.T) {
	t.Run("valid UUID", func(t *testing.T) {
		id := ID("550e8400-e29b-41d4-a716-446655440000")
		result, ok := id.UUID()
		if !ok {
			t.Fatalf("ID.UUID() ok = false, want true")
		}
		expected := "550e8400-e29b-41d4-a716-446655440000"
		if result.String() != expected {
			t.Errorf("ID.UUID() = %v, want %v", result.String(), expected)
		}
	})

	t.Run("invalid UUID slug", func(t *testing.T) {
		id := ID("my-incident-slug")
		result, ok := id.UUID()
		if ok {
			t.Errorf("ID.UUID() ok = true, want false for slug")
		}
		if result.String() != "00000000-0000-0000-0000-000000000000" {
			t.Errorf("ID.UUID() value = %v, want zero UUID for invalid input", result.String())
		}
	})

	t.Run("empty string", func(t *testing.T) {
		id := ID("")
		result, ok := id.UUID()
		if ok {
			t.Errorf("ID.UUID() ok = true, want false for empty string")
		}
		if result.String() != "00000000-0000-0000-0000-000000000000" {
			t.Errorf("ID.UUID() value = %v, want zero UUID for empty string", result.String())
		}
	})

	t.Run("malformed UUID", func(t *testing.T) {
		id := ID("not-a-uuid-at-all")
		result, ok := id.UUID()
		if ok {
			t.Errorf("ID.UUID() ok = true, want false for malformed UUID")
		}
		if result.String() != "00000000-0000-0000-0000-000000000000" {
			t.Errorf("ID.UUID() value = %v, want zero UUID for malformed input", result.String())
		}
	})

	t.Run("UUID with uppercase", func(t *testing.T) {
		id := ID("550E8400-E29B-41D4-A716-446655440000")
		result, ok := id.UUID()
		if !ok {
			t.Fatalf("ID.UUID() ok = false, want true for uppercase UUID")
		}
		// UUIDs are normalized to lowercase
		expected := "550e8400-e29b-41d4-a716-446655440000"
		if result.String() != expected {
			t.Errorf("ID.UUID() = %v, want %v", result.String(), expected)
		}
	})

	t.Run("null UUID is invalid", func(t *testing.T) {
		id := ID("00000000-0000-0000-0000-000000000000")
		result, ok := id.UUID()
		if ok {
			t.Errorf("ID.UUID() ok = true, want false for null UUID")
		}
		if result.String() != "00000000-0000-0000-0000-000000000000" {
			t.Errorf("ID.UUID() value = %v, want zero UUID", result.String())
		}
	})
}
