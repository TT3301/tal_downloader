package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/itsHenry35/tal_downloader/config"
	"github.com/itsHenry35/tal_downloader/models"
	"github.com/itsHenry35/tal_downloader/utils"
)

// GetCourseList retrieves the list of courses, paginated fetch until empty result
func (c *Client) GetCourseList() ([]*models.Course, error) {
	var allCourses []*models.Course
	page := 1
	perPage := 10

	for {
		coursesURL := fmt.Sprintf("%s/course/v1/student/course/list?stuId=%s&courseStatus=0&stdSubject=&page=%d&perPage=%d&order=desc",
			config.CourseAPIBase, c.userID, page, perPage)

		resp, err := c.doRequest("GET", coursesURL, nil, nil, false)
		if err != nil {
			utils.LogDiagnostic("course_list_error", map[string]interface{}{
				"phase": "request",
				"page":  page,
				"error": err.Error(),
			})
			return nil, err
		}
		if err := requireHTTPSuccess(resp); err != nil {
			resp.Body.Close()
			utils.LogDiagnostic("course_list_error", map[string]interface{}{
				"phase": "http_status",
				"page":  page,
				"error": err.Error(),
			})
			return nil, err
		}

		var courses []*models.Course
		if err := json.NewDecoder(resp.Body).Decode(&courses); err != nil {
			resp.Body.Close()
			utils.LogDiagnostic("course_list_error", map[string]interface{}{
				"phase": "decode",
				"page":  page,
				"error": err.Error(),
			})
			return nil, err
		}
		resp.Body.Close()
		utils.LogDiagnostic("course_list_page", map[string]interface{}{
			"page":    page,
			"count":   len(courses),
			"courses": summarizeCourses(courses),
		})

		if len(courses) == 0 {
			break // no more data
		}

		allCourses = append(allCourses, courses...)
		page++
	}

	merged := mergeDuplicateCourses(allCourses)
	utils.LogDiagnostic("course_list_merged", map[string]interface{}{
		"raw_count":     len(allCourses),
		"display_count": len(merged),
		"courses":       summarizeCourses(merged),
	})
	return merged, nil
}

func summarizeCourses(courses []*models.Course) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(courses))
	for _, course := range courses {
		if course == nil {
			continue
		}
		sources := make([]map[string]string, 0, len(course.Sources()))
		for _, source := range course.Sources() {
			sources = append(sources, map[string]string{
				"course_id": source.CourseID,
				"tutor_id":  source.TutorID,
			})
		}
		result = append(result, map[string]interface{}{
			"course_id":    course.CourseID,
			"tutor_id":     course.TutorID,
			"subject_name": course.SubjectName,
			"course_name":  course.CourseName,
			"end_live_num": course.EndLiveNum,
			"source_count": len(sources),
			"sources":      sources,
		})
	}
	return result
}

func mergeDuplicateCourses(courses []*models.Course) []*models.Course {
	merged := make([]*models.Course, 0, len(courses))
	byLogicalKey := make(map[string]*models.Course)

	for _, course := range courses {
		if course == nil {
			continue
		}
		name := strings.Join(strings.Fields(course.CourseName), " ")
		subject := strings.Join(strings.Fields(course.SubjectName), " ")
		if name == "" || course.CourseID == "" {
			merged = append(merged, course)
			continue
		}

		// endLiveNum is part of the key so two genuinely different editions with
		// the same display name but different lesson counts remain separate.
		key := fmt.Sprintf("%s\x00%s\x00%d", subject, name, course.EndLiveNum)
		if primary, exists := byLogicalKey[key]; exists {
			primary.SourceCourses = append(primary.SourceCourses, course.Sources()...)
			continue
		}

		course.SourceCourses = course.Sources()
		byLogicalKey[key] = course
		merged = append(merged, course)
	}
	return merged
}

// GetLectures retrieves lectures for a course, paginated fetch until empty result
func (c *Client) GetLectures(courseID string) ([]*models.Lecture, error) {
	var allLectures []*models.Lecture
	seenLectures := make(map[string]struct{})
	page := 1
	perPage := 10

	for {
		lecturesURL := fmt.Sprintf("%s/course/v1/student/course/user-live-list?stuId=%s&stdCourseId=%s&type=1&needPage=1&page=%d&perPage=%d&order=asc",
			config.CourseAPIBase, c.userID, courseID, page, perPage)

		resp, err := c.doRequest("GET", lecturesURL, nil, nil, false)
		if err != nil {
			return nil, err
		}
		if err := requireHTTPSuccess(resp); err != nil {
			resp.Body.Close()
			return nil, err
		}

		var lectures []*models.Lecture
		if err := json.NewDecoder(resp.Body).Decode(&lectures); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		if len(lectures) == 0 {
			break // no more data
		}

		added := 0
		for _, lecture := range lectures {
			if lecture == nil {
				continue
			}
			// Only deduplicate entries with a real live ID. Placeholder rows may
			// legitimately share empty identifiers.
			if lecture.LiveID != 0 {
				key := fmt.Sprintf("%d|%s|%s|%s", lecture.LiveID, lecture.ClassID,
					lecture.SubjectID, lecture.LiveTypeString)
				if _, exists := seenLectures[key]; exists {
					continue
				}
				seenLectures[key] = struct{}{}
			}
			lecture.ListIndex = len(allLectures) + 1
			allLectures = append(allLectures, lecture)
			added++
		}

		// A short page is the end of a normal paginated response. If an API
		// regression repeats the same page, added == 0 prevents an infinite loop.
		if len(lectures) < perPage || added == 0 {
			break
		}
		page++
	}

	return allLectures, nil
}

// GetCourseLectures joins duplicate enrollment records created by class
// transfers. The first server-ordered course entry owns each visible lecture;
// only records with a stable identity are retained as fallbacks. Unmatched
// historical lessons remain visible instead of being joined by array position.
func (c *Client) GetCourseLectures(course *models.Course) ([]*models.Lecture, error) {
	if course == nil {
		return nil, fmt.Errorf("课程信息为空")
	}

	var merged []*models.Lecture
	var failures []string

	for _, source := range course.Sources() {
		lectures, err := c.GetLectures(source.CourseID)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", source.CourseID, err))
			continue
		}
		matches := buildLectureMatchIndex(merged, lectures)
		for _, lecture := range lectures {
			if lecture == nil {
				continue
			}
			lecture.SourceCourseID = source.CourseID
			lecture.SourceTutorID = source.TutorID
			if primary := matches.find(lecture); primary != nil {
				primary.Alternates = append(primary.Alternates, lecture)
				continue
			}
			lecture.ListIndex = len(merged) + 1
			merged = append(merged, lecture)
		}
	}

	if len(merged) == 0 && len(failures) > 0 {
		return nil, fmt.Errorf("获取课程课节失败：%s", strings.Join(failures, "; "))
	}
	sort.SliceStable(merged, func(i, j int) bool {
		left := merged[i].Position
		right := merged[j].Position
		if left > 0 && right > 0 {
			return left < right
		}
		if left > 0 || right > 0 {
			return left > 0
		}
		return false
	})
	for index, lecture := range merged {
		lecture.ListIndex = index + 1
	}
	return merged, nil
}

type lectureMatchIndex struct {
	byStrongKey map[string]*models.Lecture
	byTitle     map[string]*models.Lecture
	titleCount  map[string]int
}

func buildLectureMatchIndex(existing, incoming []*models.Lecture) lectureMatchIndex {
	index := lectureMatchIndex{
		byStrongKey: make(map[string]*models.Lecture),
		byTitle:     make(map[string]*models.Lecture),
		titleCount:  make(map[string]int),
	}

	for _, lecture := range existing {
		if lecture == nil {
			continue
		}
		for _, key := range strongLectureKeys(lecture) {
			index.byStrongKey[key] = lecture
		}
		if key := lectureTitleKey(lecture); key != "" {
			index.titleCount[key]++
			index.byTitle[key] = lecture
		}
	}

	// A title is only safe as a fallback identity when it is unique on both
	// sides. Generic or repeated names must never silently connect two classes.
	incomingTitleCount := make(map[string]int)
	for _, lecture := range incoming {
		if key := lectureTitleKey(lecture); key != "" {
			incomingTitleCount[key]++
		}
	}
	for key, count := range incomingTitleCount {
		if count != 1 || index.titleCount[key] != 1 {
			delete(index.byTitle, key)
		}
	}
	return index
}

func (index lectureMatchIndex) find(lecture *models.Lecture) *models.Lecture {
	for _, key := range strongLectureKeys(lecture) {
		if primary := index.byStrongKey[key]; primary != nil {
			return primary
		}
	}
	if key := lectureTitleKey(lecture); key != "" {
		return index.byTitle[key]
	}
	return nil
}

func strongLectureKeys(lecture *models.Lecture) []string {
	if lecture == nil {
		return nil
	}
	keys := make([]string, 0, len(lecture.Tasks)+1)
	if lecture.LiveID != 0 {
		keys = append(keys, fmt.Sprintf("live:%s:%d", lecture.LiveTypeString, lecture.LiveID))
	}
	for _, task := range lecture.Tasks {
		if task.CurriculumID == "" || task.TaskID == "" || task.CoursewareID == "" {
			continue
		}
		keys = append(keys, fmt.Sprintf("task:%s:%s:%s", task.CurriculumID, task.TaskID, task.CoursewareID))
	}
	return keys
}

func lectureTitleKey(lecture *models.Lecture) string {
	if lecture == nil {
		return ""
	}
	title := strings.ToLower(strings.Join(strings.Fields(lecture.DisplayName), " "))
	if title == "" {
		return ""
	}
	return lecture.LiveTypeString + "\x00" + title
}

// GetVideoURL retrieves the download URL for a video
func (c *Client) GetVideoURL(lecture *models.Lecture, courseID, tutorID string) (string, error) {
	if lecture == nil {
		return "", fmt.Errorf("课节信息为空")
	}
	headers := map[string]string{
		"lecturerId":    lecture.LecturerID,
		"stdSubject":    lecture.SubjectID,
		"tutorId":       tutorID,
		"stdCourseId":   courseID,
		"liveId":        fmt.Sprintf("%d", lecture.LiveID),
		"liveType":      lecture.LiveTypeString,
		"stdClassId":    lecture.ClassID,
		"expireTime":    "0",
		"appClientType": "xes",
	}

	switch lecture.LiveTypeString {
	case "SMALL_GROUPS_V2_MODE", "COMBINE_SMALL_CLASS_MODE", "SMALL_CLASS_MODE", "GENERAL_V2_MODE":
		url := fmt.Sprintf("%s/playback/v1/video/init", config.ClassroomAPIBase)
		resp, err := c.doRequest("GET", url, nil, headers, false)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if err := requireHTTPSuccess(resp); err != nil {
			return "", err
		}

		var result models.VideoUrlResponse

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", err
		}

		return utils.ParseVideoUrl(result.VideoURLs, result.Message)

	case "RECORD_MODE", "ONLINE_REAL_RECORD":
		return c.getRecordVideoURL(headers)

	default:
		return "", fmt.Errorf("unsupported live type: %s", lecture.LiveTypeString)
	}

}

func (c *Client) GetCourseVideoURL(lecture *models.Lecture, course *models.Course) (string, error) {
	return c.resolveLectureVideo(lecture, course, false)
}

func (c *Client) GetCourseExtensiveVideoURL(lecture *models.Lecture, course *models.Course) (string, error) {
	return c.resolveLectureVideo(lecture, course, true)
}

func (c *Client) resolveLectureVideo(lecture *models.Lecture, course *models.Course, extensive bool) (string, error) {
	if lecture == nil || course == nil {
		return "", fmt.Errorf("课程或课节信息为空")
	}

	candidates := append([]*models.Lecture{lecture}, lecture.Alternates...)
	var failures []string
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		var (
			videoURL string
			err      error
		)
		if extensive {
			videoURL, err = c.GetExtensiveVideoURL(candidate)
		} else {
			courseID := candidate.SourceCourseID
			tutorID := candidate.SourceTutorID
			if courseID == "" {
				courseID = course.CourseID
			}
			if tutorID == "" {
				tutorID = course.TutorID
			}
			videoURL, err = c.GetVideoURL(candidate, courseID, tutorID)
		}
		if err == nil {
			return videoURL, nil
		}
		failures = append(failures, err.Error())
	}

	return "", fmt.Errorf("所有课程来源均无法解析该讲：%s", strings.Join(failures, "; "))
}

// GetExtensiveVideoURL resolves the extension-course task to its own recording
// ID. Reusing the base lecture liveId downloads the wrong lesson whenever the
// extension content is stored as ONLINE_REAL_RECORD.
func (c *Client) GetExtensiveVideoURL(lecture *models.Lecture) (string, error) {
	if lecture == nil || len(lecture.Tasks) == 0 {
		return "", fmt.Errorf("该讲没有延伸课程")
	}

	var task *models.LectureTask
	for i := range lecture.Tasks {
		candidate := &lecture.Tasks[i]
		if candidate.CurriculumID != "" && candidate.TaskID != "" && candidate.CoursewareID != "" {
			task = candidate
			break
		}
	}
	if task == nil {
		return "", fmt.Errorf("该讲的延伸课程信息不完整")
	}

	query := url.Values{}
	query.Set("curriculumId", task.CurriculumID.String())
	query.Set("taskId", task.TaskID.String())
	query.Set("taskTypeString", "ONLINE_REAL_RECORD")
	query.Set("coursewareId", task.CoursewareID.String())
	authURL := fmt.Sprintf("%s/classroom/basic/v1/real-record/init/auth?%s",
		config.ClassroomAPIBase, query.Encode())

	resp, err := c.doRequest("GET", authURL, nil, nil, false)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := requireHTTPSuccess(resp); err != nil {
		return "", err
	}

	var authResult models.OnlineRecordAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResult); err != nil {
		return "", err
	}
	if authResult.InitData.Task.RealRecordID == "" {
		return "", fmt.Errorf("未找到延伸课程录像：%s", authResult.Message)
	}

	headers := map[string]string{
		"lecturerId": lecture.LecturerID,
		"liveId":     authResult.InitData.Task.RealRecordID.String(),
		"liveType":   "ONLINE_REAL_RECORD",
	}
	return c.getRecordVideoURL(headers)
}

func (c *Client) getRecordVideoURL(headers map[string]string) (string, error) {
	recordURL := fmt.Sprintf("%s/classroom-ai/record/v1/resources", config.ClassroomAPIBase)
	resp, err := c.doRequest("GET", recordURL, nil, headers, false)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := requireHTTPSuccess(resp); err != nil {
		return "", err
	}

	var result models.RecordModeVideoUrlResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return utils.ParseRecordVideoURL(result.Definitions, result.Message)
}
