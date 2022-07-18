package scheduler

type Informer struct {
	sched *Scheduler
}

func NewInformer() *Informer {
	return &Informer{}
}

func (i *Informer) Watch() {

}
