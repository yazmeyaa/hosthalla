package dashboard_hosts

import (
	"github.com/a-h/templ"
	dashboard_hosts_ui "github.com/yazmeyaa/hosthalla/ui/widgets/dashboard_hosts/ui"
)

type HostRow = dashboard_hosts_ui.HostRow
type DashboardHostsProps = dashboard_hosts_ui.DashboardHostsProps

var DashboardHosts = dashboard_hosts_ui.DashboardHosts
var DashboardHostItemOOB = dashboard_hosts_ui.DashboardHostItemOOB

func CSSClasses() []templ.CSSClass {
	return dashboard_hosts_ui.CSSClasses()
}
