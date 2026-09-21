package ui

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/itsHenry35/tal_downloader/config"
	"github.com/itsHenry35/tal_downloader/models"
	"github.com/itsHenry35/tal_downloader/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type CourseSelectionScreen struct {
	manager           *Manager
	courseChecks      map[string]*widget.Check
	courses           []*models.Course
	downloadPath      string
	extensiveCheck    *widget.Check
	overwriteCheck    *widget.Check
	container         *fyne.Container
	courseList        *fyne.Container
	lectureSelections map[string][]*models.Lecture
	lectureCache      map[string][]*models.Lecture
	selectionSet      map[string]bool
	lectureMutex      sync.RWMutex
}

func getDownloadFolderName() string {
	return fmt.Sprintf("%s-下载", config.PlatformName)
}

func NewCourseSelectionScreen(manager *Manager) fyne.CanvasObject {
	downloadPath := filepath.Join(".", getDownloadFolderName())
	if utils.IsAndroid() {
		// 安卓使用应用存储的temp目录
		downloadPath = "temp"
	}

	cs := &CourseSelectionScreen{
		manager:           manager,
		courseChecks:      make(map[string]*widget.Check),
		lectureSelections: make(map[string][]*models.Lecture),
		lectureCache:      make(map[string][]*models.Lecture),
		selectionSet:      make(map[string]bool),
		downloadPath:      downloadPath,
	}
	cs.loadCourses()
	cs.buildUI()
	return cs.container
}

func (cs *CourseSelectionScreen) loadCourses() {
	progressDialog := dialog.NewProgressInfinite("加载中...", "正在获取课程列表", cs.manager.window)
	progressDialog.Show()

	go func() {
		defer fyne.Do(func() {
			progressDialog.Dismiss()
		})

		courses, err := cs.manager.apiClient.GetCourseList()
		if err != nil {
			utils.ShowErrorDialog(err, cs.manager.window)
			return
		}
		cs.courses = courses
		fyne.Do(func() {
			cs.updateCourseList()
		})
	}()
}

func (cs *CourseSelectionScreen) buildUI() {
	title := widget.NewLabelWithStyle("选择要下载的课程", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	// 返回按钮
	backButton := widget.NewButton("←", func() {
		cs.manager.ShowStudentSelection()
	})
	backButton.Importance = widget.LowImportance

	cs.courseList = container.NewVBox()
	scroll := container.NewScroll(cs.courseList)
	scroll.SetMinSize(fyne.NewSize(600, 400))

	cs.extensiveCheck = widget.NewCheck("下载延伸课程", nil)
	cs.overwriteCheck = widget.NewCheck("覆盖已下载文件", nil)
	scrollContainer := container.NewStack(scroll)

	// 安卓平台不显示路径选择
	var pathContainer fyne.CanvasObject
	if !utils.IsAndroid() {
		pathLabel := widget.NewLabel(fmt.Sprintf("下载路径: %s", cs.downloadPath))
		pathButton := widget.NewButton("选择路径", func() {
			dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
				if err != nil {
					utils.ShowErrorDialog(err, cs.manager.window)
					return
				}
				if uri != nil {
					cs.downloadPath = filepath.Join(uri.Path(), getDownloadFolderName())
					pathLabel.SetText(fmt.Sprintf("下载路径: %s", cs.downloadPath))
				}
			}, cs.manager.window)
		})
		pathContainer = container.NewHBox(
			layout.NewSpacer(),
			pathLabel,
			pathButton,
		)
	} else {
		pathContainer = layout.NewSpacer()
	}

	selectAllButton := widget.NewButton("全选", func() {
		for courseID, check := range cs.courseChecks {
			cs.lectureMutex.Lock()
			delete(cs.lectureSelections, courseID)
			delete(cs.selectionSet, courseID)
			cs.lectureMutex.Unlock()
			check.SetChecked(true)
		}
	})
	deselectAllButton := widget.NewButton("取消全选", func() {
		for courseID, check := range cs.courseChecks {
			check.SetChecked(false)
			cs.lectureMutex.Lock()
			delete(cs.lectureSelections, courseID)
			delete(cs.selectionSet, courseID)
			cs.lectureMutex.Unlock()
		}
	})

	downloadButton := widget.NewButton("开始下载", cs.startDownload)
	downloadButton.Importance = widget.HighImportance
	exportLogButton := widget.NewButton("导出日志", cs.exportDiagnosticLog)

	// 顶部部分（标题）
	// 使用Stack布局实现绝对定位，确保标题真正居中
	titleCentered := container.NewHBox(layout.NewSpacer(), title, layout.NewSpacer())
	backButtonContainer := container.NewHBox(backButton, layout.NewSpacer())

	titleRow := container.NewStack(titleCentered, backButtonContainer)
	top := container.NewVBox(
		container.NewPadded(titleRow),
		widget.NewSeparator(),
	)

	// 底部部分（选项与按钮）
	var optionsContainer fyne.CanvasObject
	if !utils.IsAndroid() {
		optionsContainer = container.NewVBox(
			cs.extensiveCheck,
			cs.overwriteCheck,
			pathContainer,
		)
	} else {
		optionsContainer = container.NewVBox(
			cs.extensiveCheck,
		)
	}

	bottom := container.NewVBox(
		widget.NewSeparator(),
		container.NewPadded(optionsContainer),
		container.NewPadded(
			container.NewHBox(
				selectAllButton,
				deselectAllButton,
				layout.NewSpacer(),
				exportLogButton,
				downloadButton,
			),
		),
	)

	// 中间 + 上下布局
	content := container.NewBorder(
		top, bottom, nil, nil,
		scrollContainer,
	)

	// 包一层 padding，让边缘不贴边
	cs.container = container.NewPadded(content)
}

func (cs *CourseSelectionScreen) exportDiagnosticLog() {
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			utils.ShowErrorDialog(err, cs.manager.window)
			return
		}
		if writer == nil {
			return
		}

		exportErr := utils.ExportDiagnosticLog(writer)
		closeErr := writer.Close()
		if exportErr != nil {
			utils.ShowErrorDialog(exportErr, cs.manager.window)
			return
		}
		if closeErr != nil {
			utils.ShowErrorDialog(fmt.Errorf("关闭导出文件失败: %w", closeErr), cs.manager.window)
			return
		}
		dialog.ShowInformation("导出成功", "诊断日志已导出。", cs.manager.window)
	}, cs.manager.window)
	saveDialog.SetFileName("tal_downloader_diagnostics_" + time.Now().Format("20060102-150405") + ".jsonl")
	saveDialog.Show()
}

func (cs *CourseSelectionScreen) updateCourseList() {
	cs.courseList.Objects = nil
	// 添加课程复选框
	for _, course := range cs.courses {
		courseCopy := course // 避免闭包问题

		check := widget.NewCheck(cs.courseLabel(courseCopy), func(checked bool) {
			if checked {
				return
			}
			cs.lectureMutex.Lock()
			delete(cs.lectureSelections, courseCopy.CourseID)
			delete(cs.selectionSet, courseCopy.CourseID)
			cs.lectureMutex.Unlock()
			if courseCheck, ok := cs.courseChecks[courseCopy.CourseID]; ok {
				courseCheck.SetText(cs.courseLabel(courseCopy))
			}
		})
		cs.courseChecks[course.CourseID] = check

		// 创建选择讲数的按钮
		selectLecturesBtn := widget.NewButton("...", func() {
			cs.showLectureSelectionDialog(courseCopy)
		})

		// 创建课程行
		courseRow := container.NewBorder(nil, nil, check, selectLecturesBtn)
		cs.courseList.Add(courseRow)
	}
	cs.courseList.Refresh()
}

func (cs *CourseSelectionScreen) showLectureSelectionDialog(course *models.Course) {
	progressDialog := dialog.NewProgressInfinite("加载中...", "正在获取实际课节列表", cs.manager.window)
	progressDialog.Show()

	go func() {
		lectures, err := cs.loadLectures(course)
		fyne.Do(func() {
			progressDialog.Dismiss()
			if err != nil {
				dialog.ShowError(err, cs.manager.window)
				return
			}
			cs.showLoadedLectureSelectionDialog(course, lectures)
		})
	}()
}

func (cs *CourseSelectionScreen) showLoadedLectureSelectionDialog(course *models.Course, lectures []*models.Lecture) {
	if len(lectures) == 0 {
		dialog.ShowInformation("提示", "该课程暂时没有可下载课节", cs.manager.window)
		return
	}

	cs.lectureMutex.RLock()
	selectedLectures := append([]*models.Lecture(nil), cs.lectureSelections[course.CourseID]...)
	selectionConfigured := cs.selectionSet[course.CourseID]
	cs.lectureMutex.RUnlock()

	selectedMap := make(map[string]bool)
	if selectionConfigured {
		for _, lecture := range selectedLectures {
			selectedMap[lecture.StableKey()] = true
		}
	} else if check, ok := cs.courseChecks[course.CourseID]; ok && check.Checked {
		for _, lecture := range lectures {
			selectedMap[lecture.StableKey()] = true
		}
	}

	lectureChecks := make([]*widget.Check, len(lectures))
	for i, lecture := range lectures {
		lectureChecks[i] = widget.NewCheck(lecture.Label(), nil)
		lectureChecks[i].SetChecked(selectedMap[lecture.StableKey()])
	}

	// 创建滚动容器
	checkList := container.NewVBox()
	for _, check := range lectureChecks {
		checkList.Add(check)
	}
	scroll := container.NewScroll(checkList)
	scroll.SetMinSize(fyne.NewSize(300, 400))

	// 全选和全不选按钮
	selectAllBtn := widget.NewButton("全选", func() {
		for _, check := range lectureChecks {
			check.SetChecked(true)
		}
	})

	deselectAllBtn := widget.NewButton("全不选", func() {
		for _, check := range lectureChecks {
			check.SetChecked(false)
		}
	})

	// 创建自定义对话框
	var d dialog.Dialog

	confirmBtn := widget.NewButton("确定", func() {
		// 收集选中的讲
		var selected []*models.Lecture
		for i, check := range lectureChecks {
			if check.Checked {
				selected = append(selected, lectures[i])
			}
		}
		cs.lectureMutex.Lock()
		cs.lectureSelections[course.CourseID] = selected
		cs.selectionSet[course.CourseID] = true
		cs.lectureMutex.Unlock()

		// 更新主复选框状态
		if check, ok := cs.courseChecks[course.CourseID]; ok {
			if len(selected) == 0 {
				check.SetChecked(false)
				check.SetText(cs.courseLabel(course))
			} else if len(selected) == len(lectures) {
				check.SetChecked(true)
				check.SetText(cs.courseLabel(course))
			} else {
				// 部分选中状态 - Fyne不支持三态复选框，所以保持勾选但修改文本提示
				check.SetChecked(true)
				check.Text = fmt.Sprintf("%s - %s (已选%d/%d讲)",
					course.SubjectName, course.CourseName, len(selected), len(lectures))
				check.Refresh()
			}
		}
		d.Dismiss()
	})

	cancelBtn := widget.NewButton("取消", func() {
		d.Dismiss()
	})

	buttons := container.NewHBox(
		selectAllBtn,
		deselectAllBtn,
		layout.NewSpacer(),
		cancelBtn,
		confirmBtn,
	)

	content := container.NewBorder(
		widget.NewLabel(fmt.Sprintf("选择要下载的讲 - %s", course.CourseName)),
		buttons,
		nil, nil,
		scroll,
	)

	d = dialog.NewCustomWithoutButtons("选择讲数", content, cs.manager.window)
	d.Resize(fyne.NewSize(400, 500))
	d.Show()
}

func (cs *CourseSelectionScreen) startDownload() {
	var selectedCourses []*models.Course

	for _, course := range cs.courses {
		if check, ok := cs.courseChecks[course.CourseID]; ok && check.Checked {
			selectedCourses = append(selectedCourses, course)
		}
	}

	if len(selectedCourses) == 0 {
		dialog.ShowInformation("提示", "未选择任何课程", cs.manager.window)
		return
	}

	downloadPath := cs.downloadPath
	isExtensive := cs.extensiveCheck.Checked
	isOverwrite := !utils.IsAndroid() && cs.overwriteCheck.Checked
	progressDialog := dialog.NewProgressInfinite("加载中...", "正在确认所选课节", cs.manager.window)
	progressDialog.Show()

	go func() {
		resolvedCourses := make([]*models.Course, 0, len(selectedCourses))
		resolvedLectures := make(map[string][]*models.Lecture)

		for _, course := range selectedCourses {
			course := course
			lectures, err := cs.loadLectures(course)
			if err != nil {
				fyne.Do(func() {
					progressDialog.Dismiss()
					dialog.ShowError(fmt.Errorf("%s：%w", course.CourseName, err), cs.manager.window)
				})
				return
			}

			cs.lectureMutex.RLock()
			configured := cs.selectionSet[course.CourseID]
			selected := append([]*models.Lecture(nil), cs.lectureSelections[course.CourseID]...)
			cs.lectureMutex.RUnlock()

			if configured {
				selectedKeys := make(map[string]bool, len(selected))
				for _, lecture := range selected {
					selectedKeys[lecture.StableKey()] = true
				}
				selected = selected[:0]
				for _, lecture := range lectures {
					if selectedKeys[lecture.StableKey()] {
						selected = append(selected, lecture)
					}
				}
			} else {
				selected = append([]*models.Lecture(nil), lectures...)
			}

			if len(selected) > 0 {
				resolvedCourses = append(resolvedCourses, course)
				resolvedLectures[course.CourseID] = selected
			}
		}

		if len(resolvedCourses) == 0 {
			fyne.Do(func() {
				progressDialog.Dismiss()
				dialog.ShowInformation("提示", "所选课程没有可下载课节", cs.manager.window)
			})
			return
		}
		if err := utils.Mkdir(downloadPath); err != nil {
			fyne.Do(func() {
				progressDialog.Dismiss()
				dialog.ShowError(err, cs.manager.window)
			})
			return
		}

		fyne.Do(func() {
			progressDialog.Dismiss()
			cs.manager.selectedCourses = resolvedCourses
			cs.manager.selectedLectures = resolvedLectures
			cs.manager.downloadPath = downloadPath
			cs.manager.isExtensive = isExtensive
			cs.manager.isOverwrite = isOverwrite
			cs.manager.ShowDownloadProgress()
		})
	}()
}

func (cs *CourseSelectionScreen) loadLectures(course *models.Course) ([]*models.Lecture, error) {
	if course == nil {
		return nil, fmt.Errorf("课程信息为空")
	}
	courseID := course.CourseID
	cs.lectureMutex.RLock()
	lectures, ok := cs.lectureCache[courseID]
	cs.lectureMutex.RUnlock()
	if ok {
		return lectures, nil
	}

	lectures, err := cs.manager.apiClient.GetCourseLectures(course)
	if err != nil {
		return nil, err
	}
	cs.lectureMutex.Lock()
	cs.lectureCache[courseID] = lectures
	cs.lectureMutex.Unlock()
	return lectures, nil
}

func (cs *CourseSelectionScreen) courseLabel(course *models.Course) string {
	return course.SubjectName + " - " + course.CourseName
}
