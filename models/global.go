package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type AuthData struct {
	Token    string
	UserID   string
	Nickname string
}

type Course struct {
	CourseID      string         `json:"stdCourseId"`
	TutorID       string         `json:"tutorId"`
	CourseName    string         `json:"courseName"`
	SubjectName   string         `json:"subjectName"`
	EndLiveNum    int            `json:"endLiveNum"`
	SourceCourses []CourseSource `json:"-"`
}

// CourseSource keeps every server-side enrollment behind one logical course.
// Class transfers can leave several stdCourseId values with the same course
// metadata; the first server-ordered entry remains the primary source and the
// remaining entries are available as download fallbacks.
type CourseSource struct {
	CourseID string
	TutorID  string
}

func (course *Course) Sources() []CourseSource {
	if course == nil {
		return nil
	}

	sources := make([]CourseSource, 0, len(course.SourceCourses)+1)
	seen := make(map[string]struct{})
	add := func(source CourseSource) {
		if source.CourseID == "" {
			return
		}
		key := source.CourseID + "\x00" + source.TutorID
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		sources = append(sources, source)
	}
	add(CourseSource{CourseID: course.CourseID, TutorID: course.TutorID})
	for _, source := range course.SourceCourses {
		add(source)
	}
	return sources
}

type Lecture struct {
	LiveID         int           `json:"liveId"`
	LiveTypeString string        `json:"liveTypeString"`
	ClassID        string        `json:"stdClassId"`
	SubjectID      string        `json:"stdSubject"`
	LecturerID     string        `json:"lecturerId"`
	Tasks          []LectureTask `json:"tasks"`

	// ListIndex and DisplayName are derived from the lecture-list response.
	// They let the UI keep the exact lecture selected by the user instead of
	// translating it back to a fragile array index later.
	ListIndex   int    `json:"-"`
	DisplayName string `json:"-"`

	// SourceCourseID/SourceTutorID identify the exact enrollment that produced
	// this lecture. Alternates are the same logical lecture from older transfer
	// records and are only tried when the primary source cannot resolve a video.
	SourceCourseID string     `json:"-"`
	SourceTutorID  string     `json:"-"`
	Alternates     []*Lecture `json:"-"`
}

type LectureTask struct {
	CurriculumID FlexibleID `json:"curriculumId"`
	TaskID       FlexibleID `json:"taskId"`
	CoursewareID FlexibleID `json:"coursewareId"`
}

// FlexibleID accepts both JSON strings and numbers without losing precision.
// The TAL APIs have historically used both representations for identifiers.
type FlexibleID string

func (id *FlexibleID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*id = ""
		return nil
	}

	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*id = FlexibleID(value)
		return nil
	}

	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("invalid identifier %q: %w", string(data), err)
	}
	*id = FlexibleID(number.String())
	return nil
}

func (id FlexibleID) String() string {
	return string(id)
}

func (lecture *Lecture) UnmarshalJSON(data []byte) error {
	type lectureAlias Lecture
	var decoded lectureAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*lecture = Lecture(decoded)

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	// Different TAL deployments have used different names for the lesson title.
	// Keep the wire model tolerant while still showing the server-provided title
	// whenever one is present.
	for _, key := range []string{
		"liveName", "lessonName", "lectureName", "taskName", "title", "name",
	} {
		var value string
		if raw, ok := fields[key]; ok && json.Unmarshal(raw, &value) == nil {
			value = strings.TrimSpace(value)
			if value != "" {
				lecture.DisplayName = value
				break
			}
		}
	}

	return nil
}

func (lecture *Lecture) StableKey() string {
	if lecture == nil {
		return ""
	}
	return fmt.Sprintf("%d|%s|%s|%s|%d", lecture.LiveID, lecture.ClassID,
		lecture.SubjectID, lecture.LiveTypeString, lecture.ListIndex)

}

func (lecture *Lecture) Label() string {
	if lecture == nil {
		return "未知课节"
	}

	index := lecture.ListIndex
	if index <= 0 {
		index = 1
	}
	label := fmt.Sprintf("第%d讲", index)
	if lecture.DisplayName != "" && lecture.DisplayName != label {
		label += " - " + lecture.DisplayName
	}
	return label
}

type StudentAccount struct {
	PuUID                 int    `json:"pu_uid"`
	Nickname              string `json:"nickname"`
	IsCurrentLoginAccount bool   `json:"isCurrentLoginAccount"`
}
