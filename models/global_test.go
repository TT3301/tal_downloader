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

func TestLectureUsesServerPositionForLabelAndStableKey(t *testing.T) {
	lecture := &Lecture{
		LiveID:         42,
		LiveTypeString: "SMALL_CLASS_MODE",
		ClassID:        "transferred-class",
		Position:       4,
		ListIndex:      1,
		DisplayName:    "第4讲",
	}

	if got, want := lecture.LessonNumber(), 4; got != want {
		t.Fatalf("LessonNumber() = %d, want %d", got, want)
	}
	if got, want := lecture.Label(), "第4讲"; got != want {
		t.Fatalf("Label() = %q, want %q", got, want)
	}
	if got, want := lecture.StableKey(), "42|transferred-class||SMALL_CLASS_MODE|4"; got != want {
		t.Fatalf("StableKey() = %q, want %q", got, want)
	}
}
