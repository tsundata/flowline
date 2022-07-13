package queue

import (
	"github.com/tsundata/flowline/pkg/api/meta"
)

var StageQueue = make(chan *meta.Stage, 1000)
