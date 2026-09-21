package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/itsHenry35/tal_downloader/config"
	"github.com/itsHenry35/tal_downloader/models"
)

func TestGetCourseListGroupsClassTransferRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "1" {
			fmt.Fprint(w, `[
				{"stdCourseId":"new","tutorId":"teacher-a","courseName":"【科学思维】 初一暑期JS","subjectName":"科学思维","endLiveNum":15},
				{"stdCourseId":"old-1","tutorId":"teacher-a","courseName":"【科学思维】 初一暑期JS","subjectName":"科学思维","endLiveNum":15},
				{"stdCourseId":"old-2","tutorId":"teacher-b","courseName":"【科学思维】 初一暑期JS","subjectName":"科学思维","endLiveNum":15},
				{"stdCourseId":"other","tutorId":"teacher-a","courseName":"【科学思维】 初一暑期JS","subjectName":"科学思维","endLiveNum":16}
			]`)
			return
		}
		fmt.Fprint(w, `[]`)
	}))
	defer server.Close()

	oldBase := config.CourseAPIBase
	config.CourseAPIBase = server.URL
	defer func() { config.CourseAPIBase = oldBase }()

	client := NewClient()
	client.SetAuth("test-token", "student-1")
	courses, err := client.GetCourseList()
	if err != nil {
		t.Fatalf("GetCourseList: %v", err)
	}
	if got, want := len(courses), 2; got != want {
		t.Fatalf("course count = %d, want %d", got, want)
	}
	if got, want := courses[0].CourseID, "new"; got != want {
		t.Fatalf("primary course ID = %q, want %q", got, want)
	}
	sources := courses[0].Sources()
	if got, want := len(sources), 3; got != want {
		t.Fatalf("source count = %d, want %d", got, want)
	}
	if got, want := sources[2].TutorID, "teacher-b"; got != want {
		t.Fatalf("transferred tutor ID = %q, want %q", got, want)
	}
}

func TestGetCourseLecturesKeepsTransferSourcesAsFallbacks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") != "1" {
			fmt.Fprint(w, `[]`)
			return
		}
		switch r.URL.Query().Get("stdCourseId") {
		case "new":
			fmt.Fprint(w, `[
				{"liveId":101,"liveTypeString":"RECORD_MODE","liveName":"第一讲","tasks":[{"curriculumId":"c1","taskId":"t1","coursewareId":"w1"}]},
				{"liveId":102,"liveTypeString":"RECORD_MODE","liveName":"第二讲","tasks":[{"curriculumId":"c2","taskId":"t2","coursewareId":"w2"}]}
			]`)
		case "old":
			fmt.Fprint(w, `[
				{"liveId":201,"liveTypeString":"RECORD_MODE","liveName":"第一讲","tasks":[{"curriculumId":"c1","taskId":"t1","coursewareId":"w1"}]},
				{"liveId":202,"liveTypeString":"RECORD_MODE","liveName":"第二讲","tasks":[{"curriculumId":"c2","taskId":"t2","coursewareId":"w2"}]}
			]`)
		default:
			fmt.Fprint(w, `[]`)
		}
	}))
	defer server.Close()

	oldBase := config.CourseAPIBase
	config.CourseAPIBase = server.URL
	defer func() { config.CourseAPIBase = oldBase }()

	course := &models.Course{
		CourseID: "new",
		TutorID:  "teacher-new",
		SourceCourses: []models.CourseSource{
			{CourseID: "new", TutorID: "teacher-new"},
			{CourseID: "old", TutorID: "teacher-old"},
		},
	}
	client := NewClient()
	client.SetAuth("test-token", "student-1")
	lectures, err := client.GetCourseLectures(course)
	if err != nil {
		t.Fatalf("GetCourseLectures: %v", err)
	}
	if got, want := len(lectures), 2; got != want {
		t.Fatalf("lecture count = %d, want %d", got, want)
	}
	if got, want := lectures[0].SourceCourseID, "new"; got != want {
		t.Fatalf("primary source = %q, want %q", got, want)
	}
	if got, want := len(lectures[0].Alternates), 1; got != want {
		t.Fatalf("alternate count = %d, want %d", got, want)
	}
	if got, want := lectures[0].Alternates[0].SourceCourseID, "old"; got != want {
		t.Fatalf("alternate source = %q, want %q", got, want)
	}
}

func TestGetCourseLecturesDoesNotMatchDifferentLessonsByPosition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") != "1" {
			fmt.Fprint(w, `[]`)
			return
		}
		if r.URL.Query().Get("stdCourseId") == "new" {
			fmt.Fprint(w, `[{"liveId":801,"liveTypeString":"RECORD_MODE","liveName":"第八讲"}]`)
			return
		}
		fmt.Fprint(w, `[{"liveId":101,"liveTypeString":"RECORD_MODE","liveName":"第一讲"}]`)
	}))
	defer server.Close()

	oldBase := config.CourseAPIBase
	config.CourseAPIBase = server.URL
	defer func() { config.CourseAPIBase = oldBase }()

	course := &models.Course{CourseID: "new", SourceCourses: []models.CourseSource{
		{CourseID: "new"}, {CourseID: "old"},
	}}
	client := NewClient()
	client.SetAuth("test-token", "student-1")
	lectures, err := client.GetCourseLectures(course)
	if err != nil {
		t.Fatalf("GetCourseLectures: %v", err)
	}
	if got, want := len(lectures), 2; got != want {
		t.Fatalf("lecture count = %d, want %d", got, want)
	}
	if got := len(lectures[0].Alternates); got != 0 {
		t.Fatalf("different positional lesson became an alternate: %d", got)
	}
	if got, want := lectures[1].DisplayName, "第一讲"; got != want {
		t.Fatalf("historical lesson = %q, want %q", got, want)
	}
}

func TestGetCourseLecturesOrdersTransferredLessonsByServerPosition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") != "1" {
			fmt.Fprint(w, `[]`)
			return
		}
		switch r.URL.Query().Get("stdCourseId") {
		case "current":
			fmt.Fprint(w, `[
				{"liveId":104,"liveTypeString":"SMALL_CLASS_MODE","liveName":"第4讲","pos":4},
				{"liveId":105,"liveTypeString":"SMALL_CLASS_MODE","liveName":"第5讲","pos":5}
			]`)
		case "previous-3":
			fmt.Fprint(w, `[{"liveId":103,"liveTypeString":"SMALL_CLASS_MODE","liveName":"第3讲","pos":3}]`)
		case "previous-2":
			fmt.Fprint(w, `[{"liveId":102,"liveTypeString":"SMALL_CLASS_MODE","liveName":"第2讲","pos":2}]`)
		case "previous-1":
			fmt.Fprint(w, `[{"liveId":101,"liveTypeString":"SMALL_CLASS_MODE","liveName":"第1讲","pos":1}]`)
		default:
			fmt.Fprint(w, `[]`)
		}
	}))
	defer server.Close()

	oldBase := config.CourseAPIBase
	config.CourseAPIBase = server.URL
	defer func() { config.CourseAPIBase = oldBase }()

	course := &models.Course{CourseID: "current", SourceCourses: []models.CourseSource{
		{CourseID: "current"},
		{CourseID: "previous-3"},
		{CourseID: "previous-2"},
		{CourseID: "previous-1"},
	}}
	client := NewClient()
	client.SetAuth("test-token", "student-1")

	lectures, err := client.GetCourseLectures(course)
	if err != nil {
		t.Fatalf("GetCourseLectures: %v", err)
	}
	if got, want := len(lectures), 5; got != want {
		t.Fatalf("lecture count = %d, want %d", got, want)
	}
	for index, lecture := range lectures {
		want := index + 1
		if got := lecture.Position; got != want {
			t.Fatalf("lecture %d position = %d, want %d", index, got, want)
		}
		if got := lecture.ListIndex; got != want {
			t.Fatalf("lecture %d ListIndex = %d, want %d", index, got, want)
		}
		if got := lecture.Label(); got != fmt.Sprintf("第%d讲", want) {
			t.Fatalf("lecture %d label = %q", index, got)
		}
	}
}

func TestGetCourseVideoURLFallsBackToHistoricalEnrollment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("stdCourseId") == "new" {
			http.Error(w, "not available for current enrollment", http.StatusForbidden)
			return
		}
		if got, want := r.Header.Get("tutorId"), "teacher-old"; got != want {
			t.Errorf("fallback tutor ID = %q, want %q", got, want)
		}
		fmt.Fprint(w, `{"definitions":{"高清":["https://media.example/old.mp4"]}}`)
	}))
	defer server.Close()

	oldBase := config.ClassroomAPIBase
	config.ClassroomAPIBase = server.URL
	defer func() { config.ClassroomAPIBase = oldBase }()

	primary := &models.Lecture{
		LiveID: 101, LiveTypeString: "RECORD_MODE",
		SourceCourseID: "new", SourceTutorID: "teacher-new",
	}
	primary.Alternates = []*models.Lecture{{
		LiveID: 201, LiveTypeString: "RECORD_MODE",
		SourceCourseID: "old", SourceTutorID: "teacher-old",
	}}
	course := &models.Course{CourseID: "new", TutorID: "teacher-new"}
	client := NewClient()
	client.SetAuth("test-token", "student-1")

	got, err := client.GetCourseVideoURL(primary, course)
	if err != nil {
		t.Fatalf("GetCourseVideoURL: %v", err)
	}
	if want := "https://media.example/old.mp4"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
	if strings.Contains(got, "new") {
		t.Fatalf("did not use fallback URL: %q", got)
	}
}

func TestGetLecturesKeepsStableOrderAndDeduplicatesPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case 1:
			fmt.Fprint(w, `[`)
			for i := 1; i <= 10; i++ {
				if i > 1 {
					fmt.Fprint(w, `,`)
				}
				fmt.Fprintf(w, `{"liveId":%d,"liveTypeString":"RECORD_MODE","stdClassId":"c"}`, i)
			}
			fmt.Fprint(w, `]`)
		case 2:
			fmt.Fprint(w, `[
				{"liveId":10,"liveTypeString":"RECORD_MODE","stdClassId":"c"},
				{"liveId":11,"liveTypeString":"RECORD_MODE","stdClassId":"c","liveName":"期末复习"}
			]`)
		default:
			fmt.Fprint(w, `[]`)
		}
	}))
	defer server.Close()

	oldBase := config.CourseAPIBase
	config.CourseAPIBase = server.URL
	defer func() { config.CourseAPIBase = oldBase }()

	client := NewClient()
	client.SetAuth("test-token", "student-1")
	lectures, err := client.GetLectures("course-1")
	if err != nil {
		t.Fatalf("GetLectures: %v", err)
	}
	if got, want := len(lectures), 11; got != want {
		t.Fatalf("lecture count = %d, want %d", got, want)
	}
	for i, lecture := range lectures {
		if got, want := lecture.ListIndex, i+1; got != want {
			t.Fatalf("lecture %d ListIndex = %d, want %d", i, got, want)
		}
	}
	if got, want := lectures[10].Label(), "第11讲 - 期末复习"; got != want {
		t.Fatalf("last label = %q, want %q", got, want)
	}
}

func TestGetExtensiveVideoURLUsesResolvedRecordID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/classroom/basic/v1/real-record/init/auth":
			if got := r.URL.Query().Get("taskId"); got != "task-2" {
				t.Errorf("taskId = %q", got)
			}
			fmt.Fprint(w, `{"initData":{"task":{"realRecordId":"record-999"}}}`)
		case "/classroom-ai/record/v1/resources":
			if got := r.Header.Get("liveId"); got != "record-999" {
				t.Errorf("liveId header = %q, want resolved record ID", got)
			}
			if got := r.Header.Get("liveType"); got != "ONLINE_REAL_RECORD" {
				t.Errorf("liveType header = %q", got)
			}
			fmt.Fprint(w, `{"definitions":{"高清":["https://media.example/extension.mp4"]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldBase := config.ClassroomAPIBase
	config.ClassroomAPIBase = server.URL
	defer func() { config.ClassroomAPIBase = oldBase }()

	client := NewClient()
	client.SetAuth("test-token", "student-1")
	lecture := &models.Lecture{
		LiveID:     123,
		LecturerID: "teacher-1",
		Tasks: []models.LectureTask{{
			CurriculumID: "curriculum-1",
			TaskID:       "task-2",
			CoursewareID: "courseware-3",
		}},
	}

	got, err := client.GetExtensiveVideoURL(lecture)
	if err != nil {
		t.Fatalf("GetExtensiveVideoURL: %v", err)
	}
	if want := "https://media.example/extension.mp4"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}
