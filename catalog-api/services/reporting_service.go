package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"catalogizer/database"
	"catalogizer/internal/metrics"
	"catalogizer/models"
	"catalogizer/repository"

	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
)

type ReportingService struct {
	analyticsRepo *repository.AnalyticsRepository
	userRepo      *repository.UserRepository
	db            *database.DB
}

func NewReportingService(analyticsRepo *repository.AnalyticsRepository, userRepo *repository.UserRepository, db *database.DB) *ReportingService {
	return &ReportingService{
		analyticsRepo: analyticsRepo,
		userRepo:      userRepo,
		db:            db,
	}
}

func (s *ReportingService) GenerateReport(reportType string, format string, params map[string]interface{}) (*models.GeneratedReport, error) {
	var data interface{}
	var err error

	switch reportType {
	case "user_analytics":
		data, err = s.generateUserAnalyticsData(params)
	case "system_overview":
		data, err = s.generateSystemOverviewData(params)
	case "media_analytics":
		data, err = s.generateMediaAnalyticsData(params)
	case "user_activity":
		data, err = s.generateUserActivityData(params)
	case "security_audit":
		data, err = s.generateSecurityAuditData(params)
	case "performance_metrics":
		data, err = s.generatePerformanceMetricsData(params)
	default:
		return nil, fmt.Errorf("unsupported report type: %s", reportType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate report data: %w", err)
	}

	content, err := s.formatReport(data, format, reportType)
	if err != nil {
		return nil, fmt.Errorf("failed to format report: %w", err)
	}

	report := &models.GeneratedReport{
		Type:        reportType,
		Format:      format,
		Content:     content,
		GeneratedAt: time.Now(),
		Parameters:  params,
	}

	return report, nil
}

func (s *ReportingService) generateUserAnalyticsData(params map[string]interface{}) (interface{}, error) {
	if s.userRepo == nil || s.analyticsRepo == nil {
		return nil, fmt.Errorf("repository not initialized")
	}

	userID, ok := params["user_id"].(int)
	if !ok {
		return nil, fmt.Errorf("user_id parameter required")
	}

	startDate, endDate, err := s.extractDateRange(params)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	mediaAccessLogs, err := s.analyticsRepo.GetUserMediaAccessLogs(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get media access logs: %w", err)
	}

	events, err := s.analyticsRepo.GetUserEvents(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user events: %w", err)
	}

	analytics := &models.UserAnalyticsReport{
		User:               user,
		StartDate:          startDate,
		EndDate:            endDate,
		TotalMediaAccesses: len(mediaAccessLogs),
		TotalEvents:        len(events),
		MediaAccessLogs:    mediaAccessLogs,
		Events:             events,
		AccessPatterns:     s.analyzeUserAccessPatterns(mediaAccessLogs),
		DeviceUsage:        s.analyzeUserDeviceUsage(mediaAccessLogs),
		LocationAnalysis:   s.analyzeUserLocations(mediaAccessLogs),
		TimePatterns:       s.analyzeUserTimePatterns(mediaAccessLogs),
		PopularContent:     s.analyzeUserPopularContent(mediaAccessLogs),
	}

	return analytics, nil
}

func (s *ReportingService) generateSystemOverviewData(params map[string]interface{}) (interface{}, error) {
	if s.analyticsRepo == nil {
		return nil, fmt.Errorf("analytics repository not initialized")
	}

	startDate, endDate, err := s.extractDateRange(params)
	if err != nil {
		return nil, err
	}

	totalUsers, err := s.analyticsRepo.GetTotalUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to get total users: %w", err)
	}

	activeUsers, err := s.analyticsRepo.GetActiveUsers(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}

	totalMediaAccesses, err := s.analyticsRepo.GetTotalMediaAccesses(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get total media accesses: %w", err)
	}

	totalEvents, err := s.analyticsRepo.GetTotalEvents(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get total events: %w", err)
	}

	topMedia, err := s.analyticsRepo.GetTopAccessedMedia(startDate, endDate, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to get top media: %w", err)
	}

	userGrowth, err := s.analyticsRepo.GetUserGrowthData(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user growth data: %w", err)
	}

	overview := &models.SystemOverviewReport{
		StartDate:          startDate,
		EndDate:            endDate,
		TotalUsers:         totalUsers,
		ActiveUsers:        activeUsers,
		TotalMediaAccesses: totalMediaAccesses,
		TotalEvents:        totalEvents,
		TopAccessedMedia:   topMedia,
		UserGrowthData:     userGrowth,
		SystemHealth:       s.calculateSystemHealth(totalUsers, activeUsers, totalMediaAccesses),
		UsageStatistics:    s.calculateUsageStatistics(startDate, endDate),
		PerformanceMetrics: s.calculatePerformanceMetrics(startDate, endDate),
	}

	return overview, nil
}

func (s *ReportingService) generateMediaAnalyticsData(params map[string]interface{}) (interface{}, error) {
	mediaIDFloat, ok := params["media_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("media_id parameter required")
	}
	mediaID := int(mediaIDFloat)

	startDate, endDate, err := s.extractDateRange(params)
	if err != nil {
		return nil, err
	}

	accessLogs, err := s.analyticsRepo.GetMediaAccessLogs(0, &mediaID, 10000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get media access logs: %w", err)
	}

	filteredLogs := s.filterLogsByDateRange(accessLogs, startDate, endDate)

	analytics := &models.MediaAnalyticsReport{
		MediaID:        mediaID,
		StartDate:      startDate,
		EndDate:        endDate,
		TotalAccesses:  len(filteredLogs),
		UniqueUsers:    s.countUniqueUsers(filteredLogs),
		AccessLogs:     filteredLogs,
		AccessPatterns: s.analyzeAccessPatterns(filteredLogs),
		UserEngagement: s.analyzeUserEngagement(filteredLogs),
		GeographicData: s.analyzeGeographicDistribution(filteredLogs),
		DeviceAnalysis: s.analyzeDeviceDistribution(filteredLogs),
		TimeAnalysis:   s.analyzeTimeDistribution(filteredLogs),
	}

	return analytics, nil
}

func (s *ReportingService) generateUserActivityData(params map[string]interface{}) (interface{}, error) {
	if s.analyticsRepo == nil || s.userRepo == nil {
		return nil, fmt.Errorf("repository not initialized")
	}

	startDate, endDate, err := s.extractDateRange(params)
	if err != nil {
		return nil, err
	}

	allLogs, err := s.analyticsRepo.GetAllMediaAccessLogs(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get all media access logs: %w", err)
	}

	userActivity := make(map[int][]models.MediaAccessLog)
	for _, log := range allLogs {
		userActivity[log.UserID] = append(userActivity[log.UserID], log)
	}

	var userActivities []models.UserActivitySummary
	for userID, logs := range userActivity {
		user, err := s.userRepo.GetByID(userID)
		if err != nil {
			continue
		}

		activity := models.UserActivitySummary{
			User:              user,
			TotalAccesses:     len(logs),
			LastActivity:      s.getLastActivityTime(logs),
			MostActiveHour:    s.getMostActiveHour(logs),
			PreferredDevices:  s.getPreferredDevices(logs),
			AccessedLocations: s.getAccessedLocations(logs),
		}

		userActivities = append(userActivities, activity)
	}

	report := &models.UserActivityReport{
		StartDate:      startDate,
		EndDate:        endDate,
		UserActivities: userActivities,
		TotalUsers:     len(userActivities),
		TotalAccesses:  len(allLogs),
		Summary:        s.generateActivitySummary(userActivities),
	}

	return report, nil
}

func (s *ReportingService) generateSecurityAuditData(params map[string]interface{}) (interface{}, error) {
	startDate, endDate, err := s.extractDateRange(params)
	if err != nil {
		return nil, err
	}

	failed, succeeded, err := s.loginCounts(context.Background(), startDate, endDate)
	if err != nil {
		return nil, err
	}
	audit := &models.SecurityAuditReport{
		StartDate:           startDate,
		EndDate:             endDate,
		FailedLoginAttempts: failed,
		SuccessfulLogins:    succeeded,
		SuspiciousActivity:  []models.SecurityIncident{},
		SecurityMetrics:     s.calculateSecurityMetrics(startDate, endDate),
	}

	return audit, nil
}

func (s *ReportingService) generatePerformanceMetricsData(params map[string]interface{}) (interface{}, error) {
	if s.analyticsRepo == nil {
		return nil, fmt.Errorf("analytics repository not initialized")
	}

	startDate, endDate, err := s.extractDateRange(params)
	if err != nil {
		return nil, err
	}

	sessionData, err := s.analyticsRepo.GetSessionData(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get session data: %w", err)
	}

	metrics := &models.PerformanceMetricsReport{
		StartDate:              startDate,
		EndDate:                endDate,
		AverageSessionDuration: s.calculateAverageSessionDuration(sessionData),
		TotalSessions:          len(sessionData),
		ResponseTimes:          s.calculateResponseTimes(startDate, endDate),
		SystemLoad:             s.calculateSystemLoad(startDate, endDate),
		ErrorRates:             s.calculateErrorRates(startDate, endDate),
	}

	return metrics, nil
}

func (s *ReportingService) formatReport(data interface{}, format string, reportType string) ([]byte, error) {
	switch format {
	case "json":
		return json.MarshalIndent(data, "", "  ")
	case "markdown":
		return s.formatAsMarkdown(data, reportType)
	case "html":
		return s.formatAsHTML(data, reportType)
	case "pdf":
		return s.formatAsPDF(data, reportType)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

func (s *ReportingService) formatAsMarkdown(data interface{}, reportType string) ([]byte, error) {
	var buffer bytes.Buffer

	switch reportType {
	case "user_analytics":
		report := data.(*models.UserAnalyticsReport)
		buffer.WriteString("# User Analytics Report\n\n")
		displayName := ""
		if report.User.DisplayName != nil {
			displayName = *report.User.DisplayName
		}
		buffer.WriteString(fmt.Sprintf("**User:** %s (%s)\n", displayName, report.User.Username))
		buffer.WriteString(fmt.Sprintf("**Period:** %s to %s\n\n", report.StartDate.Format("2006-01-02"), report.EndDate.Format("2006-01-02")))
		buffer.WriteString("## Summary\n\n")
		buffer.WriteString(fmt.Sprintf("- Total Media Accesses: %d\n", report.TotalMediaAccesses))
		buffer.WriteString(fmt.Sprintf("- Total Events: %d\n", report.TotalEvents))
		buffer.WriteString(fmt.Sprintf("- Account Created: %s\n\n", report.User.CreatedAt.Format("2006-01-02")))

	case "system_overview":
		report := data.(*models.SystemOverviewReport)
		buffer.WriteString("# System Overview Report\n\n")
		buffer.WriteString(fmt.Sprintf("**Period:** %s to %s\n\n", report.StartDate.Format("2006-01-02"), report.EndDate.Format("2006-01-02")))
		buffer.WriteString("## System Statistics\n\n")
		buffer.WriteString(fmt.Sprintf("- Total Users: %d\n", report.TotalUsers))
		buffer.WriteString(fmt.Sprintf("- Active Users: %d\n", report.ActiveUsers))
		buffer.WriteString(fmt.Sprintf("- Total Media Accesses: %d\n", report.TotalMediaAccesses))
		buffer.WriteString(fmt.Sprintf("- Total Events: %d\n\n", report.TotalEvents))

	default:
		buffer.WriteString(fmt.Sprintf("# %s Report\n\n", reportType))
		jsonData, _ := json.MarshalIndent(data, "", "  ")
		buffer.WriteString(fmt.Sprintf("```json\n%s\n```\n", string(jsonData)))
	}

	return buffer.Bytes(), nil
}

func (s *ReportingService) formatAsHTML(data interface{}, reportType string) ([]byte, error) {
	htmlTemplate := `
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .header { border-bottom: 2px solid #333; padding-bottom: 20px; margin-bottom: 30px; }
        .section { margin-bottom: 30px; }
        .metric { background: #f5f5f5; padding: 10px; margin: 10px 0; border-radius: 5px; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{.Title}}</h1>
        <p>Generated on: {{.GeneratedAt}}</p>
        {{if .Period}}<p>Period: {{.Period}}</p>{{end}}
    </div>

    <div class="content">
        {{.Content}}
    </div>
</body>
</html>`

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML template: %w", err)
	}

	var content string
	switch reportType {
	case "user_analytics":
		report := data.(*models.UserAnalyticsReport)
		displayNameHTML := ""
		if report.User.DisplayName != nil {
			displayNameHTML = *report.User.DisplayName
		}
		content = fmt.Sprintf(`
			<div class="section">
				<h2>User Information</h2>
				<div class="metric">Username: %s</div>
				<div class="metric">Display Name: %s</div>
				<div class="metric">Email: %s</div>
			</div>
			<div class="section">
				<h2>Activity Summary</h2>
				<div class="metric">Total Media Accesses: %d</div>
				<div class="metric">Total Events: %d</div>
			</div>`,
			report.User.Username, displayNameHTML, report.User.Email,
			report.TotalMediaAccesses, report.TotalEvents)

	case "system_overview":
		report := data.(*models.SystemOverviewReport)
		content = fmt.Sprintf(`
			<div class="section">
				<h2>System Statistics</h2>
				<div class="metric">Total Users: %d</div>
				<div class="metric">Active Users: %d</div>
				<div class="metric">Total Media Accesses: %d</div>
				<div class="metric">Total Events: %d</div>
			</div>`,
			report.TotalUsers, report.ActiveUsers, report.TotalMediaAccesses, report.TotalEvents)

	default:
		jsonData, _ := json.MarshalIndent(data, "", "  ")
		content = fmt.Sprintf("<pre>%s</pre>", string(jsonData))
	}

	templateData := struct {
		Title       string
		GeneratedAt string
		Period      string
		Content     template.HTML
	}{
		Title:       fmt.Sprintf("%s Report", reportType),
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		Content:     template.HTML(content),
	}

	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTML template: %w", err)
	}

	return buffer.Bytes(), nil
}

func (s *ReportingService) formatAsPDF(data interface{}, reportType string) ([]byte, error) {
	// Create new PDF creator
	c := creator.New()
	c.SetPageSize(creator.PageSizeLetter)

	// Load standard fonts
	arialBold, err := model.NewStandard14Font(model.HelveticaBoldName)
	if err != nil {
		return nil, fmt.Errorf("failed to load Arial Bold font: %w", err)
	}
	arialItalic, err := model.NewStandard14Font(model.HelveticaObliqueName)
	if err != nil {
		return nil, fmt.Errorf("failed to load Arial Italic font: %w", err)
	}
	courier, err := model.NewStandard14Font(model.CourierName)
	if err != nil {
		return nil, fmt.Errorf("failed to load Courier font: %w", err)
	}

	// Helper to create paragraph
	createParagraph := func(text string, font *model.PdfFont, fontSize float64, x, y float64) error {
		para := &creator.Paragraph{}
		para.SetText(text)
		para.SetFont(font)
		para.SetFontSize(fontSize)
		para.SetColor(creator.ColorBlack)
		para.SetPos(x, y)
		return c.Draw(para)
	}

	// Set title
	if err := createParagraph(
		fmt.Sprintf("%s Report", strings.Replace(reportType, "_", " ", -1)),
		arialBold, 16, 50, 50,
	); err != nil {
		return nil, fmt.Errorf("failed to draw title: %w", err)
	}

	// Add generation timestamp
	if err := createParagraph(
		fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")),
		arialItalic, 10, 50, 70,
	); err != nil {
		return nil, fmt.Errorf("failed to draw timestamp: %w", err)
	}

	// Format content based on report type
	switch reportType {
	case "user_analytics":
		return s.formatUserAnalyticsPDF(c, data, arialBold, arialItalic, courier)
	case "system_overview":
		return s.formatSystemOverviewPDF(c, data, arialBold, arialItalic, courier)
	case "media_analytics":
		return s.formatMediaAnalyticsPDF(c, data, arialBold, arialItalic, courier)
	case "user_activity":
		return s.formatUserActivityPDF(c, data, arialBold, arialItalic, courier)
	case "security_audit":
		return s.formatSecurityAuditPDF(c, data, arialBold, arialItalic, courier)
	case "performance_metrics":
		return s.formatPerformanceMetricsPDF(c, data, arialBold, arialItalic, courier)
	default:
		// Fallback to JSON representation
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data for PDF: %w", err)
		}

		// Split JSON into lines and add to PDF
		lines := strings.Split(string(jsonData), "\n")
		y := 90.0
		for _, line := range lines {
			// Truncate long lines to fit page
			if len(line) > 80 {
				for i := 0; i < len(line); i += 80 {
					end := i + 80
					if end > len(line) {
						end = len(line)
					}
					if err := createParagraph(line[i:end], courier, 10, 50, y); err != nil {
						return nil, fmt.Errorf("failed to draw JSON line: %w", err)
					}
					y += 12
				}
			} else {
				if err := createParagraph(line, courier, 10, 50, y); err != nil {
					return nil, fmt.Errorf("failed to draw JSON line: %w", err)
				}
				y += 12
			}
		}

		// Output PDF to bytes
		var buf bytes.Buffer
		if err := c.Write(&buf); err != nil {
			return nil, fmt.Errorf("failed to generate PDF: %w", err)
		}
		return buf.Bytes(), nil
	}
}

// Helper methods for specific report types
func (s *ReportingService) formatUserAnalyticsPDF(c *creator.Creator, data interface{}, arialBold, arialItalic, courier *model.PdfFont) ([]byte, error) {
	report := data.(*models.UserAnalyticsReport)

	// Helper to create paragraph
	createParagraph := func(text string, font *model.PdfFont, fontSize float64, x, y float64) error {
		para := &creator.Paragraph{}
		para.SetText(text)
		para.SetFont(font)
		para.SetFontSize(fontSize)
		para.SetColor(creator.ColorBlack)
		para.SetPos(x, y)
		return c.Draw(para)
	}

	y := 90.0

	// User Information heading
	if err := createParagraph("User Information", arialBold, 12, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw heading: %w", err)
	}
	y += 15

	// User details
	if err := createParagraph(fmt.Sprintf("User ID: %d", report.User.ID), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw user id: %w", err)
	}
	y += 12
	if err := createParagraph(fmt.Sprintf("Username: %s", report.User.Username), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw username: %w", err)
	}
	y += 12
	if err := createParagraph(fmt.Sprintf("Email: %s", report.User.Email), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw email: %w", err)
	}
	y += 12
	if err := createParagraph(fmt.Sprintf("Period: %s to %s",
		report.StartDate.Format("2006-01-02"),
		report.EndDate.Format("2006-01-02")), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw period: %w", err)
	}
	y += 20

	// Summary Statistics heading
	if err := createParagraph("Summary Statistics", arialBold, 12, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw summary heading: %w", err)
	}
	y += 15

	// Statistics
	if err := createParagraph(fmt.Sprintf("Total Media Accesses: %d", report.TotalMediaAccesses), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw total media accesses: %w", err)
	}
	y += 12
	if err := createParagraph(fmt.Sprintf("Total Events: %d", report.TotalEvents), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw total events: %w", err)
	}
	y += 12
	if err := createParagraph(fmt.Sprintf("Access Logs: %d", len(report.MediaAccessLogs)), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw access logs: %w", err)
	}
	y += 12
	if err := createParagraph(fmt.Sprintf("User Events: %d", len(report.Events)), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw user events: %w", err)
	}

	// Output PDF to bytes
	var buf bytes.Buffer
	if err := c.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}
	return buf.Bytes(), nil
}

func (s *ReportingService) formatSystemOverviewPDF(c *creator.Creator, data interface{}, arialBold, arialItalic, courier *model.PdfFont) ([]byte, error) {
	report := data.(*models.SystemOverviewReport)

	// Helper to create paragraph
	createParagraph := func(text string, font *model.PdfFont, fontSize float64, x, y float64) error {
		para := &creator.Paragraph{}
		para.SetText(text)
		para.SetFont(font)
		para.SetFontSize(fontSize)
		para.SetColor(creator.ColorBlack)
		para.SetPos(x, y)
		return c.Draw(para)
	}

	y := 90.0

	// System Statistics heading
	if err := createParagraph("System Statistics", arialBold, 12, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw heading: %w", err)
	}
	y += 15

	// Statistics
	if err := createParagraph(fmt.Sprintf("Total Users: %d", report.TotalUsers), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw total users: %w", err)
	}
	y += 12
	if err := createParagraph(fmt.Sprintf("Active Users: %d", report.ActiveUsers), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw active users: %w", err)
	}
	y += 12
	if err := createParagraph(fmt.Sprintf("Total Media Accesses: %d", report.TotalMediaAccesses), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw total media accesses: %w", err)
	}
	y += 12
	if err := createParagraph(fmt.Sprintf("Total Events: %d", report.TotalEvents), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw total events: %w", err)
	}
	y += 12
	if err := createParagraph(fmt.Sprintf("Report Period: %s to %s",
		report.StartDate.Format("2006-01-02"),
		report.EndDate.Format("2006-01-02")), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw report period: %w", err)
	}

	// Output PDF to bytes
	var buf bytes.Buffer
	if err := c.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}
	return buf.Bytes(), nil
}

func (s *ReportingService) formatMediaAnalyticsPDF(c *creator.Creator, data interface{}, arialBold, arialItalic, courier *model.PdfFont) ([]byte, error) {
	// Helper to create paragraph
	createParagraph := func(text string, font *model.PdfFont, fontSize float64, x, y float64) error {
		para := &creator.Paragraph{}
		para.SetText(text)
		para.SetFont(font)
		para.SetFontSize(fontSize)
		para.SetColor(creator.ColorBlack)
		para.SetPos(x, y)
		return c.Draw(para)
	}

	y := 90.0

	// Media Analytics heading
	if err := createParagraph("Media Analytics", arialBold, 12, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw heading: %w", err)
	}
	y += 15

	// Description
	if err := createParagraph("Media statistics and analytics data", arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw description: %w", err)
	}
	y += 15

	// Add JSON data for now (can be enhanced later)
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal media analytics data: %w", err)
	}

	lines := strings.Split(string(jsonData), "\n")
	for _, line := range lines {
		if len(line) > 100 {
			line = line[:100] + "..."
		}
		if err := createParagraph(line, courier, 10, 50, y); err != nil {
			return nil, fmt.Errorf("failed to draw JSON line: %w", err)
		}
		y += 12
	}

	// Output PDF to bytes
	var buf bytes.Buffer
	if err := c.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}
	return buf.Bytes(), nil
}

func (s *ReportingService) formatUserActivityPDF(c *creator.Creator, data interface{}, arialBold, arialItalic, courier *model.PdfFont) ([]byte, error) {
	// Helper to create paragraph
	createParagraph := func(text string, font *model.PdfFont, fontSize float64, x, y float64) error {
		para := &creator.Paragraph{}
		para.SetText(text)
		para.SetFont(font)
		para.SetFontSize(fontSize)
		para.SetColor(creator.ColorBlack)
		para.SetPos(x, y)
		return c.Draw(para)
	}

	y := 90.0

	// User Activity Report heading
	if err := createParagraph("User Activity Report", arialBold, 12, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw heading: %w", err)
	}
	y += 15

	// Description
	if err := createParagraph("User activity logs and events", arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw description: %w", err)
	}
	y += 15

	// Add summary information
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user activity data: %w", err)
	}

	text := string(jsonData)
	if len(text) > 3000 { // Limit text to reasonable size
		text = text[:3000] + "...[truncated]"
	}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if len(line) > 100 {
			line = line[:100] + "..."
		}
		if err := createParagraph(line, courier, 10, 50, y); err != nil {
			return nil, fmt.Errorf("failed to draw JSON line: %w", err)
		}
		y += 12
	}

	// Output PDF to bytes
	var buf bytes.Buffer
	if err := c.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}
	return buf.Bytes(), nil
}

func (s *ReportingService) formatSecurityAuditPDF(c *creator.Creator, data interface{}, arialBold, arialItalic, courier *model.PdfFont) ([]byte, error) {
	// Helper to create paragraph
	createParagraph := func(text string, font *model.PdfFont, fontSize float64, x, y float64) error {
		para := &creator.Paragraph{}
		para.SetText(text)
		para.SetFont(font)
		para.SetFontSize(fontSize)
		para.SetColor(creator.ColorBlack)
		para.SetPos(x, y)
		return c.Draw(para)
	}

	y := 90.0

	// Security Audit Report heading
	if err := createParagraph("Security Audit Report", arialBold, 12, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw heading: %w", err)
	}
	y += 15

	// Subheading
	if err := createParagraph("Security Events and Audit Information", arialBold, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw subheading: %w", err)
	}
	y += 12

	// Description
	if err := createParagraph("This report contains security audit information", arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw description: %w", err)
	}
	y += 12

	// Generation timestamp
	if err := createParagraph(fmt.Sprintf("Generated on: %s", time.Now().Format("2006-01-02 15:04:05")), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw timestamp: %w", err)
	}
	y += 15

	// Add audit data as formatted JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal security audit data: %w", err)
	}

	lines := strings.Split(string(jsonData), "\n")
	for _, line := range lines {
		if len(line) > 95 {
			line = line[:95] + "..."
		}
		if err := createParagraph(line, courier, 10, 50, y); err != nil {
			return nil, fmt.Errorf("failed to draw JSON line: %w", err)
		}
		y += 12
	}

	// Output PDF to bytes
	var buf bytes.Buffer
	if err := c.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}
	return buf.Bytes(), nil
}

func (s *ReportingService) formatPerformanceMetricsPDF(c *creator.Creator, data interface{}, arialBold, arialItalic, courier *model.PdfFont) ([]byte, error) {
	// Helper to create paragraph
	createParagraph := func(text string, font *model.PdfFont, fontSize float64, x, y float64) error {
		para := &creator.Paragraph{}
		para.SetText(text)
		para.SetFont(font)
		para.SetFontSize(fontSize)
		para.SetColor(creator.ColorBlack)
		para.SetPos(x, y)
		return c.Draw(para)
	}

	y := 90.0

	// Performance Metrics Report heading
	if err := createParagraph("Performance Metrics Report", arialBold, 12, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw heading: %w", err)
	}
	y += 15

	// Subheading
	if err := createParagraph("System Performance Metrics", arialBold, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw subheading: %w", err)
	}
	y += 12

	// Description
	if err := createParagraph("This report contains performance metrics for the system", arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw description: %w", err)
	}
	y += 12

	// Generation timestamp
	if err := createParagraph(fmt.Sprintf("Generated on: %s", time.Now().Format("2006-01-02 15:04:05")), arialItalic, 10, 50, y); err != nil {
		return nil, fmt.Errorf("failed to draw timestamp: %w", err)
	}
	y += 15

	// Add performance data as formatted JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal performance metrics data: %w", err)
	}

	lines := strings.Split(string(jsonData), "\n")
	for _, line := range lines {
		if len(line) > 95 {
			line = line[:95] + "..."
		}
		if err := createParagraph(line, courier, 10, 50, y); err != nil {
			return nil, fmt.Errorf("failed to draw JSON line: %w", err)
		}
		y += 12
	}

	// Output PDF to bytes
	var buf bytes.Buffer
	if err := c.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}
	return buf.Bytes(), nil
}

func (s *ReportingService) extractDateRange(params map[string]interface{}) (time.Time, time.Time, error) {
	// Article XI §11.5: handlers/service_handlers.go::GetUsage/Performance
	// pre-parse query strings into time.Time and pass them in
	// the params map. The previous string-only type assertion saw
	// time.Time, missed the assertion, and reported
	// "start_date parameter required" even though the handler had
	// already validated the parameter. Caught by FQA-API-277/278 in
	// the 2026-04-29 real-binary bank verification.
	startDate, err := paramAsDate(params, "start_date")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endDate, err := paramAsDate(params, "end_date")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return startDate, endDate, nil
}

// paramAsDate accepts the value associated with `key` as either
// a parsed time.Time, a *time.Time, or a YYYY-MM-DD string —
// returns the time.Time. Missing keys produce
// "<key> parameter required" so callers can distinguish missing
// from malformed.
func paramAsDate(params map[string]interface{}, key string) (time.Time, error) {
	v, present := params[key]
	if !present {
		return time.Time{}, fmt.Errorf("%s parameter required", key)
	}
	switch t := v.(type) {
	case time.Time:
		return t, nil
	case *time.Time:
		if t == nil {
			return time.Time{}, fmt.Errorf("%s parameter required", key)
		}
		return *t, nil
	case string:
		if t == "" {
			return time.Time{}, fmt.Errorf("%s parameter required", key)
		}
		parsed, err := time.Parse("2006-01-02", t)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid %s format", key)
		}
		return parsed, nil
	default:
		return time.Time{}, fmt.Errorf("invalid %s type %T (expected string or time.Time)", key, v)
	}
}

// Helper methods for analytics calculations
func (s *ReportingService) analyzeUserAccessPatterns(logs []models.MediaAccessLog) map[string]interface{} {
	patterns := make(map[string]interface{})

	hourlyAccess := make(map[int]int)
	dailyAccess := make(map[string]int)

	for _, log := range logs {
		hour := log.AccessTime.Hour()
		hourlyAccess[hour]++

		day := log.AccessTime.Weekday().String()
		dailyAccess[day]++
	}

	patterns["hourly"] = hourlyAccess
	patterns["daily"] = dailyAccess

	return patterns
}

func (s *ReportingService) analyzeUserDeviceUsage(logs []models.MediaAccessLog) map[string]int {
	deviceUsage := make(map[string]int)

	for _, log := range logs {
		if log.DeviceInfo != nil {
			deviceModel := ""
			if log.DeviceInfo.DeviceModel != nil {
				deviceModel = *log.DeviceInfo.DeviceModel
			}
			platform := ""
			if log.DeviceInfo.Platform != nil {
				platform = *log.DeviceInfo.Platform
			}
			device := fmt.Sprintf("%s %s", platform, deviceModel)
			deviceUsage[device]++
		}
	}

	return deviceUsage
}

func (s *ReportingService) analyzeUserLocations(logs []models.MediaAccessLog) map[string]int {
	locations := make(map[string]int)

	for _, log := range logs {
		if log.Location != nil {
			location := fmt.Sprintf("%.2f,%.2f", log.Location.Latitude, log.Location.Longitude)
			locations[location]++
		}
	}

	return locations
}

func (s *ReportingService) analyzeUserTimePatterns(logs []models.MediaAccessLog) map[string]interface{} {
	return s.analyzeUserAccessPatterns(logs) // Same as access patterns
}

func (s *ReportingService) analyzeUserPopularContent(logs []models.MediaAccessLog) []models.MediaAccessCount {
	mediaCount := make(map[int]int)

	for _, log := range logs {
		mediaCount[log.MediaID]++
	}

	var results []models.MediaAccessCount
	for mediaID, count := range mediaCount {
		results = append(results, models.MediaAccessCount{
			MediaID:     mediaID,
			AccessCount: count,
		})
	}

	return results
}

func (s *ReportingService) filterLogsByDateRange(logs []models.MediaAccessLog, startDate, endDate time.Time) []models.MediaAccessLog {
	var filtered []models.MediaAccessLog

	for _, log := range logs {
		if log.AccessTime.After(startDate) && log.AccessTime.Before(endDate) {
			filtered = append(filtered, log)
		}
	}

	return filtered
}

func (s *ReportingService) countUniqueUsers(logs []models.MediaAccessLog) int {
	users := make(map[int]bool)

	for _, log := range logs {
		users[log.UserID] = true
	}

	return len(users)
}

func (s *ReportingService) calculateSystemHealth(totalUsers, activeUsers, mediaAccesses int) models.SystemHealth {
	var healthScore float64

	if totalUsers > 0 {
		activeRatio := float64(activeUsers) / float64(totalUsers)
		healthScore += activeRatio * 50
	}

	if mediaAccesses > 0 {
		healthScore += 30
	}

	if activeUsers > 10 {
		healthScore += 20
	}

	var status string
	switch {
	case healthScore >= 80:
		status = "excellent"
	case healthScore >= 60:
		status = "good"
	case healthScore >= 40:
		status = "fair"
	default:
		status = "poor"
	}

	return models.SystemHealth{
		Score:  healthScore,
		Status: status,
	}
}

func (s *ReportingService) calculateUsageStatistics(startDate, endDate time.Time) models.UsageStatistics {
	if s.analyticsRepo == nil {
		return models.UsageStatistics{}
	}

	// Get actual access logs to calculate peak hours
	logs, err := s.analyticsRepo.GetAllMediaAccessLogs(startDate, endDate)
	if err != nil {
		return models.UsageStatistics{}
	}

	// Calculate peak hours from actual data
	hourCounts := make(map[int]int)
	for _, log := range logs {
		hour := log.AccessTime.Hour()
		hourCounts[hour]++
	}

	// Find top 5 peak hours
	type hourCount struct {
		hour  int
		count int
	}
	var hours []hourCount
	for h, c := range hourCounts {
		hours = append(hours, hourCount{h, c})
	}
	// Sort by count descending
	for i := 0; i < len(hours); i++ {
		for j := i + 1; j < len(hours); j++ {
			if hours[j].count > hours[i].count {
				hours[i], hours[j] = hours[j], hours[i]
			}
		}
	}

	peakHours := []int{}
	for i := 0; i < 5 && i < len(hours); i++ {
		peakHours = append(peakHours, hours[i].hour)
	}

	// Calculate average daily accesses
	days := int(endDate.Sub(startDate).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	avgDaily := int(float64(len(logs)) / float64(days))

	// Growth rate: compare the current window's access count to the same
	// window ending at startDate. Positive = growth, negative = decline.
	growthRate := 0.0
	windowDuration := endDate.Sub(startDate)
	if windowDuration > 0 {
		prevStart := startDate.Add(-windowDuration)
		prevEnd := startDate
		prevLogs, err := s.analyticsRepo.GetAllMediaAccessLogs(prevStart, prevEnd)
		if err == nil && len(prevLogs) > 0 {
			growthRate = (float64(len(logs)) - float64(len(prevLogs))) / float64(len(prevLogs)) * 100
		}
	}

	return models.UsageStatistics{
		PeakHours:    peakHours,
		AverageDaily: avgDaily,
		GrowthRate:   growthRate,
	}
}

func (s *ReportingService) calculatePerformanceMetrics(startDate, endDate time.Time) models.PerformanceMetrics {
	// Read current HTTP metrics from the Prometheus registry. The metrics
	// package's GinMiddleware instruments every request so the histogram /
	// counter children are populated automatically. The snapshot reflects
	// cumulative totals since process start — callers interested in a
	// specific interval must subtract two snapshots themselves.
	snap := metrics.SnapshotHTTP()
	var errorRate float64
	if snap.TotalRequests > 0 {
		errorRate = float64(snap.Errors4xx+snap.Errors5xx) / float64(snap.TotalRequests) * 100
	}
	// Throughput = requests-per-second averaged over the reporting window.
	var throughput int
	if window := endDate.Sub(startDate).Seconds(); window > 0 {
		throughput = int(float64(snap.TotalRequests) / window)
	}
	return models.PerformanceMetrics{
		ResponseTime: snap.P95Seconds * 1000, // ms
		Throughput:   throughput,
		ErrorRate:    errorRate,
	}
}

func (s *ReportingService) analyzeAccessPatterns(logs []models.MediaAccessLog) map[string]interface{} {
	return s.analyzeUserAccessPatterns(logs)
}

func (s *ReportingService) analyzeUserEngagement(logs []models.MediaAccessLog) models.UserEngagement {
	if len(logs) == 0 {
		return models.UserEngagement{}
	}

	// Calculate actual session times from logs
	var totalSessionTime float64
	userSessions := make(map[int64][]time.Duration) // userID -> session durations
	var lastLogTime time.Time
	var currentUserID int64

	for i, log := range logs {
		userID := int64(log.UserID)
		if i == 0 || userID != currentUserID {
			currentUserID = userID
		}
		// Estimate session time from consecutive logs (within 30 min = same session)
		if i > 0 && userID == currentUserID && log.AccessTime.Sub(lastLogTime) < 30*time.Minute {
			sessionDuration := log.AccessTime.Sub(lastLogTime).Minutes()
			userSessions[userID] = append(userSessions[userID], time.Duration(sessionDuration)*time.Minute)
			totalSessionTime += sessionDuration
		}
		lastLogTime = log.AccessTime
	}

	// Calculate average session time
	avgSessionTime := totalSessionTime / float64(len(logs))

	// Calculate return rate (% of users with multiple sessions)
	usersWithMultipleSessions := 0
	for _, sessions := range userSessions {
		if len(sessions) > 1 {
			usersWithMultipleSessions++
		}
	}
	returnRate := 0.0
	if len(userSessions) > 0 {
		returnRate = float64(usersWithMultipleSessions) / float64(len(userSessions)) * 100
	}

	// Calculate interaction depth (avg actions per session)
	var totalInteractions int
	for _, sessions := range userSessions {
		totalInteractions += len(sessions)
	}
	interactionDepth := 0.0
	if len(userSessions) > 0 {
		interactionDepth = float64(totalInteractions) / float64(len(userSessions))
	}

	return models.UserEngagement{
		AverageSessionTime: avgSessionTime,
		ReturnRate:         returnRate,
		InteractionDepth:   interactionDepth,
	}
}

func (s *ReportingService) analyzeGeographicDistribution(logs []models.MediaAccessLog) map[string]int {
	return s.analyzeUserLocations(logs)
}

func (s *ReportingService) analyzeDeviceDistribution(logs []models.MediaAccessLog) map[string]int {
	return s.analyzeUserDeviceUsage(logs)
}

func (s *ReportingService) analyzeTimeDistribution(logs []models.MediaAccessLog) map[string]int {
	timeDistribution := make(map[string]int)

	for _, log := range logs {
		hour := log.AccessTime.Hour()
		var timeSlot string

		switch {
		case hour >= 6 && hour < 12:
			timeSlot = "morning"
		case hour >= 12 && hour < 18:
			timeSlot = "afternoon"
		case hour >= 18 && hour < 22:
			timeSlot = "evening"
		default:
			timeSlot = "night"
		}

		timeDistribution[timeSlot]++
	}

	return timeDistribution
}

func (s *ReportingService) getLastActivityTime(logs []models.MediaAccessLog) time.Time {
	if len(logs) == 0 {
		return time.Time{}
	}

	latest := logs[0].AccessTime
	for _, log := range logs {
		if log.AccessTime.After(latest) {
			latest = log.AccessTime
		}
	}

	return latest
}

func (s *ReportingService) getMostActiveHour(logs []models.MediaAccessLog) int {
	hourCounts := make(map[int]int)

	for _, log := range logs {
		hour := log.AccessTime.Hour()
		hourCounts[hour]++
	}

	maxCount := 0
	mostActiveHour := 0

	for hour, count := range hourCounts {
		if count > maxCount {
			maxCount = count
			mostActiveHour = hour
		}
	}

	return mostActiveHour
}

func (s *ReportingService) getPreferredDevices(logs []models.MediaAccessLog) []string {
	deviceCounts := s.analyzeUserDeviceUsage(logs)

	var devices []string
	for device := range deviceCounts {
		devices = append(devices, device)
	}

	return devices
}

func (s *ReportingService) getAccessedLocations(logs []models.MediaAccessLog) []string {
	locationCounts := s.analyzeUserLocations(logs)

	var locations []string
	for location := range locationCounts {
		locations = append(locations, location)
	}

	return locations
}

func (s *ReportingService) generateActivitySummary(activities []models.UserActivitySummary) models.ActivitySummary {
	if len(activities) == 0 {
		return models.ActivitySummary{}
	}

	totalAccesses := 0
	for _, activity := range activities {
		totalAccesses += activity.TotalAccesses
	}

	avgAccesses := float64(totalAccesses) / float64(len(activities))

	return models.ActivitySummary{
		TotalUsers:       len(activities),
		TotalAccesses:    totalAccesses,
		AverageAccesses:  avgAccesses,
		MostActiveUsers:  len(activities), // Simplified
		LeastActiveUsers: 0,               // Simplified
	}
}

// loginCounts reads failed and successful logins in [startDate, endDate] from
// auth_audit_log, the table internal/auth writes ("failed_login",
// "failed_login_inactive", "login_success"). An unreadable audit trail is an
// error: it must never be reported as zero failed logins.
func (s *ReportingService) loginCounts(ctx context.Context, startDate, endDate time.Time) (failed, succeeded int, err error) {
	if s.db == nil {
		return 0, 0, fmt.Errorf("read auth_audit_log: database not initialized")
	}
	const layout = "2006-01-02 15:04:05" // the CURRENT_TIMESTAMP format the column holds
	err = s.db.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(CASE WHEN event_type LIKE 'failed_login%' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN event_type = 'login_success' THEN 1 ELSE 0 END), 0)
		FROM auth_audit_log WHERE created_at BETWEEN ? AND ?`,
		startDate.UTC().Format(layout), endDate.UTC().Format(layout)).Scan(&failed, &succeeded)
	if err != nil {
		return 0, 0, fmt.Errorf("read auth_audit_log: %w", err)
	}
	return failed, succeeded, nil
}

func (s *ReportingService) calculateSecurityMetrics(startDate, endDate time.Time) models.SecurityMetrics {
	ctx := context.Background()
	var unavailable []string

	failedLogins, _, err := s.loginCounts(ctx, startDate, endDate)
	failedKnown := err == nil
	if !failedKnown {
		unavailable = append(unavailable, "failed_logins")
	}

	// High/critical unresolved alerts are the vulnerability signal. Nothing in
	// this codebase writes system_alerts, so the count is usually unmeasurable;
	// that is reported as unavailable, never as a clean zero.
	vulnerabilityCount := 0
	vulnKnown := false
	if s.db != nil {
		row := s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM system_alerts WHERE severity IN ('high','critical') AND resolved = false AND created_at BETWEEN ? AND ?",
			startDate, endDate)
		vulnKnown = row.Scan(&vulnerabilityCount) == nil
	}
	if !vulnKnown {
		vulnerabilityCount = 0
		unavailable = append(unavailable, "vulnerability_count")
	}

	if !failedKnown {
		// The main input is unmeasurable: no score, no threat verdict.
		return models.SecurityMetrics{ThreatLevel: "unknown", Unavailable: append(unavailable, "security_score")}
	}

	// Calculate security score based on actual metrics
	score := 100.0
	if failedLogins > 0 {
		score -= float64(failedLogins) * 0.5
	}
	if vulnerabilityCount > 0 {
		score -= float64(vulnerabilityCount) * 5.0
	}
	if score < 0 {
		score = 0
	}

	threatLevel := "low"
	if score < 50 {
		threatLevel = "critical"
	} else if score < 70 {
		threatLevel = "high"
	} else if score < 85 {
		threatLevel = "medium"
	}

	return models.SecurityMetrics{
		ThreatLevel:        threatLevel,
		VulnerabilityCount: vulnerabilityCount,
		SecurityScore:      score,
		Unavailable:        unavailable,
	}
}

func (s *ReportingService) calculateAverageSessionDuration(sessions []models.SessionData) time.Duration {
	if len(sessions) == 0 {
		return 0
	}

	var total time.Duration
	for _, session := range sessions {
		total += session.Duration
	}

	return total / time.Duration(len(sessions))
}

func (s *ReportingService) calculateResponseTimes(startDate, endDate time.Time) models.ResponseTimes {
	// Read from the Prometheus HTTP histogram (already populated by
	// metrics.GinMiddleware). Percentiles are approximated via linear
	// interpolation of the bucket boundaries.
	snap := metrics.SnapshotHTTP()
	var avg float64
	if snap.TotalRequests > 0 {
		avg = snap.TotalDurationSeconds / float64(snap.TotalRequests) * 1000
	}
	return models.ResponseTimes{
		Average: avg,
		Min:     snap.MinSeconds * 1000,
		Max:     snap.MaxSeconds * 1000,
		P95:     snap.P95Seconds * 1000,
		P99:     snap.P99Seconds * 1000,
	}
}

func (s *ReportingService) calculateSystemLoad(startDate, endDate time.Time) models.SystemLoad {
	// Use Go runtime stats rather than /proc or gopsutil (neither is
	// imported, and gopsutil pulls in platform-specific CGO). Memory is
	// reported as "percentage of Go heap use vs. OS-requested memory".
	sys := metrics.SnapshotSystem()
	var memPct float64
	if sys.SysBytes > 0 {
		memPct = float64(sys.HeapInuse) / float64(sys.SysBytes) * 100
	}
	// We don't have direct CPU / disk / network samplers — report 0.0
	// with Memory populated from the runtime. Phase 6 of the completion
	// roadmap wires a full system metrics collector that populates these.
	return models.SystemLoad{
		CPU:     0,
		Memory:  memPct,
		Disk:    0,
		Network: 0,
	}
}

func (s *ReportingService) calculateErrorRates(startDate, endDate time.Time) models.ErrorRates {
	// Reads from the HTTPRequestsTotal counter grouped by status label.
	// Values are absolute since process start; callers interested in a
	// window must diff two snapshots.
	snap := metrics.SnapshotHTTP()
	var p4xx, p5xx, total float64
	if snap.TotalRequests > 0 {
		t := float64(snap.TotalRequests)
		p4xx = float64(snap.Errors4xx) / t * 100
		p5xx = float64(snap.Errors5xx) / t * 100
		total = p4xx + p5xx
	}
	return models.ErrorRates{
		HTTP4xx:  p4xx,
		HTTP5xx:  p5xx,
		Timeouts: 0, // not distinguished in the current instrumentation
		Total:    total,
	}
}
