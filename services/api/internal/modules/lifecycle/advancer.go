package lifecycle

import "context"

type Advancer struct{ svc *Service }

func NewAdvancer(svc *Service) *Advancer { return &Advancer{svc: svc} }

func (a *Advancer) Run(ctx context.Context) error {
	phases, err := a.svc.repo.ListPhasesForAutoAdvance(ctx)
	if err != nil {
		return err
	}
	for _, phase := range phases {
		if err := a.svc.CompletePhase(ctx, phase.ID); err != nil {
			continue
		}
		if err := a.svc.AdvanceToNextPhase(ctx, phase.LifecycleID); err != nil {
			continue
		}
	}
	return nil
}
