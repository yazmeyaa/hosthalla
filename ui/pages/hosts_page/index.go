package hosts_page

import (
	"github.com/a-h/templ"
	hosts_page_model "github.com/yazmeyaa/hosthalla/ui/pages/hosts_page/model"
	hosts_page_ui "github.com/yazmeyaa/hosthalla/ui/pages/hosts_page/ui"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/badge"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/card"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/checkbox"
)

type HostLatestMetricsBadges = hosts_page_model.HostLatestMetricsBadges
type HostsPageProps = hosts_page_ui.HostsPageProps
type PingResult = hosts_page_ui.PingResult

var BuildHostLatestMetricsBadges = hosts_page_model.BuildHostLatestMetricsBadges
var HostsPage = hosts_page_ui.HostsPage
var HostsPageContent = hosts_page_ui.HostsPageContent
var HostPingResult = hosts_page_ui.HostPingResult
var HostPingResultSlot = hosts_page_ui.HostPingResultSlot
var HostPingResultsBatch = hosts_page_ui.HostPingResultsBatch

func CSSClasses() []templ.CSSClass {
	classes := hosts_page_ui.CSSClasses()
	classes = append(classes, card.CSSClasses()...)
	classes = append(classes, badge.CSSClasses()...)
	classes = append(classes, checkbox.CSSClasses()...)
	return classes
}
