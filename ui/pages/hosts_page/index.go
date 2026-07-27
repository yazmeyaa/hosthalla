package hosts_page

import (
	hosts_page_model "github.com/yazmeyaa/hosthalla/ui/pages/hosts_page/model"
	hosts_page_ui "github.com/yazmeyaa/hosthalla/ui/pages/hosts_page/ui"
)

type HostLatestMetricsBadges = hosts_page_model.HostLatestMetricsBadges
type HostsPageProps = hosts_page_ui.HostsPageProps
type PingResult = hosts_page_ui.PingResult

var BuildHostLatestMetricsBadges = hosts_page_model.BuildHostLatestMetricsBadges
var HostsPage = hosts_page_ui.HostsPage
var HostsPageContent = hosts_page_ui.HostsPageContent
var HostPingResult = hosts_page_ui.HostPingResult
var HostPingResultsBatch = hosts_page_ui.HostPingResultsBatch
