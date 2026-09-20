package models

import (
	"encoding/json"
	"testing"
)

func TestLectureUnmarshalPreservesIDsAndDisplayName(t *testing.T) {
	var lecture Lecture
	payload := []byte(`{
		"liveId": 42,
		"liveTypeString": "RECORD_MODE",
		"stdClassId": "class-1",
		"stdSubject": "math",
		"lecturerId": "teacher-1",
		"lessonName": "函数专题",
		"tasks": [{"curriculumId": 9007199254740993, "taskId": "task-2", "coursewareId": 3}]
	}`)

	if err := json.Unmarshal(payload, &lecture); err != nil {
		t.Fatalf("unmarshal lecture: %v", err)
	}
	lecture.ListIndex = 7

	if got, want := lecture.DisplayName, "函数专题"; got != want {
		t.Fatalf("DisplayName = %q, want %q", got, want)
	}
	if got, want := lecture.Label(), "第7讲 - 函数专题"; got != want {
		t.Fatalf("Label() = %q, want %q", got, want)
	}
	if got, want := lecture.Tasks[0].CurriculumID.String(), "9007199254740993"; got != want {
		t.Fatalf("CurriculumID = %q, want %q", got, want)
	}
	if got, want := lecture.Tasks[0].TaskID.String(), "task-2"; got != want {
		t.Fatalf("TaskID = %q, want %q", got, want)
	}
}
