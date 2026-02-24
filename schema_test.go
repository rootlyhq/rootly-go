package rootly

import "testing"

// https://github.com/rootlyhq/rootly-go/issues/39
func Test_OptionalQueryParameterIsNotSerializedIntoURL(t *testing.T) {
	params := ListShiftsParams{
		ScheduleIDs: []string{"schedule-1", "schedule-2"},
		UserIDs:     nil,
	}

	req, err := NewListShiftsRequest(ServerURLProduction, &params)
	if err != nil {
		t.Errorf("Expected no error when calling NewListShiftsRequest, but received %v", err)
	}

	want := "https://api.rootly.com/v1/shifts?schedule_ids%5B%5D=schedule-1&schedule_ids%5B%5D=schedule-2"
	got := req.URL.String()

	if want != got {
		t.Errorf("ListShift API URL was not correct. Expected:\n%v\nReceived:\n%v\n", want, got)
	}
}
