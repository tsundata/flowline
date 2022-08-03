package plugins

import (
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/scheduler/framework/config"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/names"
)

var DefaultProfile = []config.Profile{
	{
		SchedulerName: constant.DefaultSchedulerName,
		Plugins: &config.Plugins{
			QueueSort: config.PluginSet{
				Enabled: []config.Plugin{
					{Name: names.PrioritySort, Weight: 1},
				},
				Disabled: nil,
			},
			Filter: config.PluginSet{
				Enabled: []config.Plugin{
					{Name: names.WorkerRuntime, Weight: 1},
				},
				Disabled: nil,
			},
			Score: config.PluginSet{
				Enabled: []config.Plugin{
					{Name: names.DefaultScore, Weight: 1},
				},
				Disabled: nil,
			},
			Permit: config.PluginSet{
				Enabled: []config.Plugin{
					{Name: names.DefaultPermit, Weight: 1},
				},
				Disabled: nil,
			},
			Bind: config.PluginSet{
				Enabled: []config.Plugin{
					{Name: names.DefaultBinder, Weight: 1},
				},
				Disabled: nil,
			},
		},
		PluginConfig: nil,
	},
}
