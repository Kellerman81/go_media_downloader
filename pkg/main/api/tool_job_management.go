package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// renderJobManagementPage renders a page for starting jobs.
func renderJobManagementPage(csrfToken string) gomponents.Node {
	mediaConfigs := jobMediaConfigs()

	// The job type dropdown is refilled for the chosen configuration by
	// jobListScript; the first configuration's types are the initial ones.
	var jobTypes []string
	if len(mediaConfigs) > 0 {
		jobTypes = jobTypesFor(config.GetSettingsMedia(mediaConfigs[0]))
	}

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fa-solid fa-tasks header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Job Management")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Start single jobs for media processing, data refreshing, and searching. Jobs will be queued and executed based on your configuration settings.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("jobForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-4"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Job Configuration")),

					renderFormGroup("job", map[string]string{
						"JobType": "Select the type of job to run",
					}, map[string]string{
						"JobType": "Job Type",
					}, "JobType", "select", "data", map[string][]string{
						"options": jobTypes,
					}),

					renderFormGroup("job", map[string]string{
						"MediaConfig": "Media configuration to use for the job",
					}, map[string]string{
						"MediaConfig": "Media Configuration",
					}, "MediaConfig", "select", "", map[string][]string{
						"options": mediaConfigs,
					}),
				),

				html.Div(
					html.Class("col-md-4"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Job Options")),

					renderFormGroup("job", map[string]string{
						"ListName": "Optional: limit the job to one list of the media configuration. " +
							"Only used by " + strings.Join(jobTypesWithList, ", ") + ".",
					}, map[string]string{
						"ListName": "List Name",
					}, "ListName", "select", "", map[string][]string{
						"options": {""},
					}),

					renderFormGroup("job", map[string]string{
						"Force": "Force execution even if scheduler is disabled",
					}, map[string]string{
						"Force": "Force Execution",
					}, "Force", "checkbox", false, nil),
				),

				html.Div(
					html.Class("col-md-4"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Job Descriptions")),
					html.Details(
						html.Summary(gomponents.Text("Job Type Descriptions")),
						html.Ul(
							html.Li(
								html.Strong(gomponents.Text("data: ")),
								gomponents.Text("Scan and import new media files"),
							),
							html.Li(
								html.Strong(gomponents.Text("datafull: ")),
								gomponents.Text("Full rescan of all media files"),
							),
							html.Li(
								html.Strong(gomponents.Text("feeds: ")),
								gomponents.Text("Refresh RSS feeds and lists"),
							),
							html.Li(
								html.Strong(gomponents.Text("rss: ")),
								gomponents.Text("Process RSS downloads"),
							),
							html.Li(
								html.Strong(gomponents.Text("rssseasons/rssseasonsall: ")),
								gomponents.Text("Season-specific RSS processing"),
							),
							html.Li(
								html.Strong(gomponents.Text("checkmissing: ")),
								gomponents.Text("Check for missing episodes/movies"),
							),
							html.Li(
								html.Strong(gomponents.Text("search*: ")),
								gomponents.Text(
									"Various search operations for missing/upgrade content",
								),
							),
							html.Li(
								html.Strong(gomponents.Text("clearhistory: ")),
								gomponents.Text("Clear download history"),
							),
						),
					),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-primary"),
					gomponents.Text("Start Job"),
					html.Type("button"),
					hx.Target("#jobResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/jobmanagement"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#jobForm"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('jobForm').reset(); document.getElementById('jobResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("jobResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),

		jobListScript(mediaConfigs),

		// Instructions
		html.Div(
			html.Class("mt-4 card border-0 shadow-sm border-info mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-info me-3"),
						html.I(html.Class("fas fa-info-circle me-1")),
						gomponents.Text("Usage"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text("Usage Instructions"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text("Follow these steps to execute management jobs"),
				),
				html.Ol(
					html.Class("mb-0 list-unstyled"),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("1. Select the job type you want to execute"),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"2. Choose the media configuration (required for most jobs)",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"3. Optionally specify a list name for targeted processing",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"4. Check 'Force Execution' to run even if scheduler is disabled",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("5. Click 'Start Job' to queue the job for execution"),
					),
				),
			),
		),

		html.Div(
			html.Class("card border-0 shadow-sm border-info mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-info me-3"),
						html.I(html.Class("fas fa-lightbulb me-1")),
						gomponents.Text("Note"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text("Important Information"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					html.Class("card-text text-muted mb-0"),
					gomponents.Text(
						"Jobs are queued and executed asynchronously. Check the scheduler page for job status and history.",
					),
				),
			),
		),
	)
}

// jobTypesWithList are the job types that can be limited to one list
// (utils.SingleJobs); the others always run for the whole media config.
var jobTypesWithList = []string{
	"feeds", "checkmissing", "checkmissingflag", "checkupgradeflag", "checkreachedflag", "clearhistory",
}

// jobMediaConfigs returns the config key (e.g. "movie_EN") of every media
// configuration, all media types.
func jobMediaConfigs() []string {
	media := config.GetSettingsMediaAll()
	if media == nil {
		return nil
	}

	var out []string

	for _, group := range [][]config.MediaTypeConfig{
		media.Movies, media.Series, media.Music, media.Books, media.AudioBooks,
	} {
		for i := range group {
			out = append(out, group[i].NamePrefix)
		}
	}

	return out
}

// jobTypesFor returns the job types utils.SingleJobs runs for cfgp's media
// type: the media type's scheduler jobs plus data and the generic flag and
// history jobs.
func jobTypesFor(cfgp *config.MediaTypeConfig) []string {
	if cfgp == nil {
		return nil
	}

	types := []string{logger.StrData}

	if h := mediatype.Get(cfgp.IsType); h != nil {
		for _, pair := range h.GetSchedulerJobNames() {
			if !slices.Contains(types, pair[1]) {
				types = append(types, pair[1])
			}
		}
	}

	for _, generic := range []string{logger.StrReachedFlag, logger.StrClearHistory} {
		if !slices.Contains(types, generic) {
			types = append(types, generic)
		}
	}

	return types
}

// jobListOption is one entry of the List Name dropdown.
type jobListOption struct {
	Name     string `json:"name"`
	Disabled bool   `json:"disabled"`
}

// jobListScript fills the List Name dropdown with the lists of the chosen
// media configuration and disables it for job types that take no list.
func jobListScript(mediaConfigs []string) gomponents.Node {
	lists := make(map[string][]jobListOption, len(mediaConfigs))
	jobs := make(map[string][]string, len(mediaConfigs))

	for _, name := range mediaConfigs {
		cfgp := config.GetSettingsMedia(name)
		if cfgp == nil {
			continue
		}

		jobs[name] = jobTypesFor(cfgp)

		for _, l := range cfgp.Lists {
			if l.Name != "" {
				lists[name] = append(lists[name], jobListOption{Name: l.Name, Disabled: !l.Enabled})
			}
		}
	}

	data, _ := json.Marshal(map[string]any{"lists": lists, "jobs": jobs, "withList": jobTypesWithList})

	return html.Script(gomponents.Raw(`
		(function() {
			var cfg = ` + string(data) + `;
			var form = document.getElementById('jobForm');
			if (!form) return;
			var media = form.querySelector('[name="job_MediaConfig"]');
			var job = form.querySelector('[name="job_JobType"]');
			var list = form.querySelector('[name="job_ListName"]');
			if (!media || !job || !list) return;

			function fill() {
				var keep = list.value;
				list.innerHTML = '';
				var all = document.createElement('option');
				all.value = '';
				all.textContent = 'All lists';
				list.appendChild(all);
				(cfg.lists[media.value] || []).forEach(function(l) {
					var o = document.createElement('option');
					o.value = l.name;
					o.textContent = l.disabled ? l.name + ' (disabled)' : l.name;
					list.appendChild(o);
				});
				list.value = keep;
				if (list.value !== keep) list.value = '';
				toggle();
			}

			function fillJobs() {
				var keep = job.value;
				job.innerHTML = '';
				(cfg.jobs[media.value] || []).forEach(function(j) {
					var o = document.createElement('option');
					o.value = j;
					o.textContent = j;
					job.appendChild(o);
				});
				job.value = keep;
				if (job.value !== keep && job.options && job.options.length) job.value = job.options[0].value;
			}

			function toggle() {
				var used = cfg.withList.indexOf(job.value) >= 0;
				list.disabled = !used;
				if (!used) list.value = '';
			}

			media.addEventListener('change', function() { fillJobs(); fill(); });
			job.addEventListener('change', toggle);
			form.addEventListener('reset', function() { setTimeout(function() { fillJobs(); fill(); }, 0); });
			fillJobs();
			fill();
		})();
	`))
}

// HandleJobManagement handles job start requests.
func HandleJobManagement(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	jobType := c.PostForm("job_JobType")
	mediaConfig := c.PostForm("job_MediaConfig")
	listName := c.PostForm("job_ListName")
	force := formBool(c.PostForm("job_Force"))

	if jobType == "" {
		c.String(http.StatusOK, renderAlert("Please select a job type", "warning"))
		return
	}

	if mediaConfig == "" {
		c.String(http.StatusOK, renderAlert("Please select a media configuration", "warning"))
		return
	}

	cfgp := config.GetSettingsMedia(mediaConfig)
	if cfgp == nil {
		c.String(http.StatusOK, renderAlert("Unknown media configuration "+mediaConfig, "warning"))
		return
	}

	if !slices.Contains(jobTypesFor(cfgp), jobType) {
		c.String(http.StatusOK, renderAlert("Job "+jobType+" does not exist for "+mediaConfig, "warning"))
		return
	}

	// SingleJobs silently does nothing for a list the config doesn't have.
	if listName != "" {
		if _, ok := cfgp.ListsMapIdx[listName]; !ok {
			c.String(http.StatusOK, renderAlert("Media configuration "+mediaConfig+" has no list "+listName, "warning"))
			return
		}
	}

	// Start the job using worker.Dispatch
	err := worker.Dispatch(jobType+"_"+mediaConfig, func(key uint32, ctx context.Context) error {
		return utils.SingleJobs(ctx, jobType, mediaConfig, listName, force, key)
	}, "Data")
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to start job: "+err.Error(), "danger"))
		return
	}

	result := html.Div(
		html.Class("card border-0 shadow-sm border-success mb-4"),

		html.Div(
			html.Class("card-header border-0"),
			html.Style(
				"background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;",
			),
			html.Div(
				html.Class("d-flex align-items-center"),
				html.Span(
					html.Class("badge bg-success me-3"),
					html.I(html.Class("fas fa-play-circle me-1")),
					gomponents.Text("Started"),
				),
				html.H5(
					html.Class("card-title mb-0 text-success fw-bold"),
					gomponents.Text("Job Started Successfully"),
				),
			),
		),

		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("card border-0 mb-3"),
				html.Style("background: rgba(40, 167, 69, 0.05); border-radius: 8px;"),
				html.Div(
					html.Class("card-body p-3"),
					html.P(
						html.Class("mb-3"),
						html.Style("color: #495057;"),
						gomponents.Text(
							fmt.Sprintf(
								"Job '%s' has been queued for execution with the following parameters:",
								jobType,
							),
						),
					),

					html.Div(
						html.Class("row g-2 mb-3"),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style(
									"background: rgba(40, 167, 69, 0.1); border-radius: 6px;",
								),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(
										html.Class("d-flex align-items-center"),
										html.I(
											html.Class("fas fa-cog me-2"),
											html.Style("color: #28a745;"),
										),
										html.Div(
											html.Div(
												html.Class("small fw-bold text-muted"),
												gomponents.Text("Job Type"),
											),
											html.Div(
												html.Class("fw-semibold"),
												gomponents.Text(jobType),
											),
										),
									),
								),
							),
						),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style(
									"background: rgba(40, 167, 69, 0.1); border-radius: 6px;",
								),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(
										html.Class("d-flex align-items-center"),
										html.I(
											html.Class("fas fa-film me-2"),
											html.Style("color: #28a745;"),
										),
										html.Div(
											html.Div(
												html.Class("small fw-bold text-muted"),
												gomponents.Text("Media Config"),
											),
											html.Div(
												html.Class("fw-semibold"),
												gomponents.Text(mediaConfig),
											),
										),
									),
								),
							),
						),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style(
									"background: rgba(40, 167, 69, 0.1); border-radius: 6px;",
								),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(
										html.Class("d-flex align-items-center"),
										html.I(
											html.Class("fas fa-list me-2"),
											html.Style("color: #28a745;"),
										),
										html.Div(
											html.Div(
												html.Class("small fw-bold text-muted"),
												gomponents.Text("List Name"),
											),
											html.Div(
												html.Class("fw-semibold"),
												func() gomponents.Node {
													if listName != "" {
														return gomponents.Text(listName)
													}

													return gomponents.Text("(all lists)")
												}(),
											),
										),
									),
								),
							),
						),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style(
									"background: rgba(40, 167, 69, 0.1); border-radius: 6px;",
								),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(
										html.Class("d-flex align-items-center"),
										html.I(
											html.Class("fas fa-bolt me-2"),
											html.Style("color: #28a745;"),
										),
										html.Div(
											html.Div(
												html.Class("small fw-bold text-muted"),
												gomponents.Text("Force Execution"),
											),
											html.Div(
												html.Class("fw-semibold"),
												gomponents.Text(fmt.Sprintf("%t", force)),
											),
										),
									),
								),
							),
						),
					),
				),
			),

			html.Div(
				html.Class("card border-0 mb-0"),
				html.Style("background-color: rgba(40, 167, 69, 0.1); border-radius: 8px;"),
				html.Div(
					html.Class("card-body p-3"),
					html.Div(
						html.Class("d-flex align-items-start"),
						html.I(
							html.Class("fas fa-info-circle me-2 mt-1"),
							html.Style("color: #28a745; font-size: 0.9rem;"),
						),
						html.Small(
							html.Style("color: #495057; line-height: 1.4;"),
							html.Strong(gomponents.Text("Status: ")),
							gomponents.Text(
								"The job is now running in the background. You can monitor its progress in the scheduler interface.",
							),
						),
					),
				),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}
