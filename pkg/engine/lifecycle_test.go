// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package engine

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cancel"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/workspace"
)

type JournalEntryKind int

const (
	JournalEntryBegin   JournalEntryKind = 0
	JournalEntrySuccess JournalEntryKind = 1
	JournalEntryFailure JournalEntryKind = 2
	JournalEntryOutputs JournalEntryKind = 4
)

type JournalEntry struct {
	Kind JournalEntryKind
	Step deploy.Step
}

type Journal struct {
	Entries []JournalEntry
	events  chan JournalEntry
	cancel  chan bool
	done    chan bool
}

func (j *Journal) Close() error {
	close(j.cancel)
	close(j.events)
	<-j.done

	return nil
}

func (j *Journal) BeginMutation(step deploy.Step) (SnapshotMutation, error) {
	select {
	case j.events <- JournalEntry{Kind: JournalEntryBegin, Step: step}:
		return j, nil
	case <-j.cancel:
		return nil, errors.New("journal closed")
	}
}

func (j *Journal) End(step deploy.Step, success bool) error {
	kind := JournalEntryFailure
	if success {
		kind = JournalEntrySuccess
	}
	select {
	case j.events <- JournalEntry{Kind: kind, Step: step}:
		return nil
	case <-j.cancel:
		return errors.New("journal closed")
	}
}

func (j *Journal) RegisterResourceOutputs(step deploy.Step) error {
	select {
	case j.events <- JournalEntry{Kind: JournalEntryOutputs, Step: step}:
		return nil
	case <-j.cancel:
		return errors.New("journal closed")
	}
}

func (j *Journal) RecordPlugin(plugin workspace.PluginInfo) error {
	return nil
}

func (j *Journal) Snap(base *deploy.Snapshot) *deploy.Snapshot {
	// Build up a list of current resources by replaying the journal.
	resources, dones := []*resource.State{}, make(map[*resource.State]bool)
	ops, doneOps := []resource.Operation{}, make(map[*resource.State]bool)
	for _, e := range j.Entries {
		logging.V(7).Infof("%v %v (%v)", e.Step.Op(), e.Step.URN(), e.Kind)

		// Begin journal entries add pending operations to the snapshot. As we see success or failure
		// entries, we'll record them in doneOps.
		if e.Kind == JournalEntryBegin {
			switch e.Step.Op() {
			case deploy.OpCreate, deploy.OpCreateReplacement:
				ops = append(ops, resource.NewOperation(e.Step.New(), resource.OperationTypeCreating))
			case deploy.OpDelete, deploy.OpDeleteReplaced:
				ops = append(ops, resource.NewOperation(e.Step.Old(), resource.OperationTypeDeleting))
			case deploy.OpRead, deploy.OpReadReplacement:
				ops = append(ops, resource.NewOperation(e.Step.New(), resource.OperationTypeReading))
			case deploy.OpUpdate:
				ops = append(ops, resource.NewOperation(e.Step.New(), resource.OperationTypeUpdating))
			}

			continue
		}

		if e.Kind != JournalEntryOutputs {
			switch e.Step.Op() {
			case deploy.OpCreate, deploy.OpCreateReplacement, deploy.OpRead, deploy.OpReadReplacement, deploy.OpUpdate:
				doneOps[e.Step.New()] = true
			case deploy.OpDelete, deploy.OpDeleteReplaced:
				doneOps[e.Step.Old()] = true
			}
		}

		if e.Kind != JournalEntrySuccess {
			continue
		}

		switch e.Step.Op() {
		case deploy.OpSame, deploy.OpUpdate:
			resources = append(resources, e.Step.New())
			dones[e.Step.Old()] = true
		case deploy.OpCreate, deploy.OpCreateReplacement:
			resources = append(resources, e.Step.New())
		case deploy.OpDelete, deploy.OpDeleteReplaced:
			dones[e.Step.Old()] = true
		case deploy.OpReplace:
			// do nothing.
		case deploy.OpRead, deploy.OpReadReplacement:
			resources = append(resources, e.Step.New())
			if e.Step.Old() != nil {
				dones[e.Step.Old()] = true
			}
		}
	}

	// Append any resources from the base snapshot that were not produced by the current snapshot.
	// See backend.SnapshotManager.snap for why this works.
	if base != nil {
		for _, res := range base.Resources {
			if !dones[res] {
				resources = append(resources, res)
			}
		}
	}

	// Append any pending operations.
	var operations []resource.Operation
	for _, op := range ops {
		if !doneOps[op.Resource] {
			operations = append(operations, op)
		}
	}

	manifest := deploy.Manifest{}
	manifest.Magic = manifest.NewMagic()
	return deploy.NewSnapshot(manifest, resources, operations)
}

func newJournal() *Journal {
	j := &Journal{
		events: make(chan JournalEntry),
		cancel: make(chan bool),
		done:   make(chan bool),
	}
	go func() {
		for e := range j.events {
			j.Entries = append(j.Entries, e)
		}
		close(j.done)
	}()
	return j
}

type updateInfo struct {
	project workspace.Project
	target  deploy.Target
}

func (u *updateInfo) GetRoot() string {
	return ""
}

func (u *updateInfo) GetProject() *workspace.Project {
	return &u.project
}

func (u *updateInfo) GetTarget() *deploy.Target {
	return &u.target
}

type TestOp func(UpdateInfo, *Context, UpdateOptions, bool) (ResourceChanges, error)
type ValidateFunc func(project workspace.Project, target deploy.Target, j *Journal, err error) error

func (op TestOp) Run(project workspace.Project, target deploy.Target, opts UpdateOptions,
	dryRun bool, validate ValidateFunc) (*deploy.Snapshot, error) {

	// Create an appropriate update info and context.
	info := &updateInfo{project: project, target: target}

	cancelCtx, _ := cancel.NewContext(context.Background())
	events := make(chan Event)
	journal := newJournal()

	ctx := &Context{
		Cancel:          cancelCtx,
		Events:          events,
		SnapshotManager: journal,
	}

	// Begin draining events.
	go func() {
		for range events {
		}
	}()

	// Run the step and its validator.
	_, err := op(info, ctx, opts, dryRun)
	contract.IgnoreClose(journal)

	if dryRun {
		return nil, err
	}
	if validate != nil {
		err = validate(project, target, journal, err)
	}

	snap := journal.Snap(target.Snapshot)
	if snap != nil {
		err = snap.VerifyIntegrity()
	}
	return snap, err
}

type TestStep struct {
	Op       TestOp
	Validate ValidateFunc
}

type TestPlan struct {
	Project   string
	Stack     string
	Runtime   string
	Config    config.Map
	Decrypter config.Decrypter
	Options   UpdateOptions
	Steps     []TestStep
}

func (p *TestPlan) getNames() (stack tokens.QName, project tokens.PackageName, runtime string) {
	project = tokens.PackageName(p.Project)
	if project == "" {
		project = "test"
	}
	runtime = p.Runtime
	if runtime == "" {
		runtime = "test"
	}
	stack = tokens.QName(p.Stack)
	if stack == "" {
		stack = "test"
	}
	return stack, project, runtime
}

func (p *TestPlan) NewURN(typ tokens.Type, name string, parent resource.URN) resource.URN {
	stack, project, _ := p.getNames()
	var pt tokens.Type
	if parent != "" {
		pt = parent.Type()
	}
	return resource.NewURN(stack, project, pt, typ, tokens.QName(name))
}

func (p *TestPlan) NewProviderURN(pkg tokens.Package, name string, parent resource.URN) resource.URN {
	return p.NewURN(providers.MakeProviderType(pkg), name, parent)
}

func (p *TestPlan) Run(t *testing.T, snapshot *deploy.Snapshot) *deploy.Snapshot {
	stack, projectName, runtime := p.getNames()

	cfg := p.Config
	if cfg == nil {
		cfg = config.Map{}
	}

	project := &workspace.Project{
		Name:        projectName,
		RuntimeInfo: workspace.NewProjectRuntimeInfo(runtime, nil),
	}
	target := &deploy.Target{
		Name:      stack,
		Config:    cfg,
		Decrypter: p.Decrypter,
		Snapshot:  snapshot,
	}

	for _, step := range p.Steps {
		_, err := step.Op.Run(*project, *target, p.Options, true, step.Validate)
		assert.NoError(t, err)
		target.Snapshot, err = step.Op.Run(*project, *target, p.Options, false, step.Validate)
		assert.NoError(t, err)
	}

	return target.Snapshot
}

func MakeBasicLifecycleSteps(t *testing.T, resCount int) []TestStep {
	return []TestStep{
		// Initial update
		{
			Op: Update,
			Validate: func(project workspace.Project, target deploy.Target, j *Journal, err error) error {
				// Should see only creates.
				for _, entry := range j.Entries {
					assert.Equal(t, deploy.OpCreate, entry.Step.Op())
				}
				assert.Len(t, j.Snap(target.Snapshot).Resources, resCount)
				return err
			},
		},
		// No-op refresh
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, j *Journal, err error) error {
				// Should see only sames.
				for _, entry := range j.Entries {
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				}
				assert.Len(t, j.Snap(target.Snapshot).Resources, resCount)
				return err
			},
		},
		// No-op update
		{
			Op: Update,
			Validate: func(project workspace.Project, target deploy.Target, j *Journal, err error) error {
				// Should see only sames.
				for _, entry := range j.Entries {
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				}
				assert.Len(t, j.Snap(target.Snapshot).Resources, resCount)
				return err
			},
		},
		// No-op refresh
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, j *Journal, err error) error {
				// Should see only sames.
				for _, entry := range j.Entries {
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				}
				assert.Len(t, j.Snap(target.Snapshot).Resources, resCount)
				return err
			},
		},
		// Destroy
		{
			Op: Destroy,
			Validate: func(project workspace.Project, target deploy.Target, j *Journal, err error) error {
				// Should see only deletes.
				for _, entry := range j.Entries {
					assert.Equal(t, deploy.OpDelete, entry.Step.Op())
				}
				assert.Len(t, j.Snap(target.Snapshot).Resources, 0)
				return err
			},
		},
		// No-op refresh
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, j *Journal, err error) error {
				assert.Len(t, j.Entries, 0)
				assert.Len(t, j.Snap(target.Snapshot).Resources, 0)
				return err
			},
		},
	}
}

func TestEmptyProgramLifecycle(t *testing.T) {
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	host := deploytest.NewPluginHost(nil, program)

	p := &TestPlan{
		Options: UpdateOptions{host: host},
		Steps:   MakeBasicLifecycleSteps(t, 0),
	}
	p.Run(t, nil)
}

func TestSingleResourceDefaultProviderLifecycle(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, "", false, nil, "",
			resource.PropertyMap{})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{host: host},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)
}

func TestSingleResourceExplicitProviderLifecycle(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true, "",
			false, nil, "", resource.PropertyMap{})
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, "", false, nil, provRef.String(),
			resource.PropertyMap{})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{host: host},
		Steps:   MakeBasicLifecycleSteps(t, 2),
	}
	p.Run(t, nil)
}

func TestSingleResourceDefaultProviderUpgrade(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, "", false, nil, "",
			resource.PropertyMap{})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{host: host},
	}

	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Create an old snapshot with an existing copy of the single resource and no providers.
	old := &deploy.Snapshot{
		Resources: []*resource.State{{
			Type:    resURN.Type(),
			URN:     resURN,
			Custom:  true,
			ID:      "0",
			Inputs:  resource.PropertyMap{},
			Outputs: resource.PropertyMap{},
		}},
	}

	validate := func(project workspace.Project, target deploy.Target, j *Journal, err error) error {
		// Should see only sames: the default provider should be injected into the old state before the update
		// runs.
		for _, entry := range j.Entries {
			switch urn := entry.Step.URN(); urn {
			case provURN, resURN:
				assert.Equal(t, deploy.OpSame, entry.Step.Op())
			default:
				t.Fatalf("unexpected resource %v", urn)
			}
		}
		assert.Len(t, j.Snap(target.Snapshot).Resources, 2)
		return err
	}

	// Run a single update step using the base snapshot.
	p.Steps = []TestStep{{Op: Update, Validate: validate}}
	p.Run(t, old)

	// Run a single refresh step using the base snapshot.
	p.Steps = []TestStep{{Op: Refresh, Validate: validate}}
	p.Run(t, old)

	// Run a single destroy step using the base snapshot.
	p.Steps = []TestStep{{
		Op: Destroy,
		Validate: func(project workspace.Project, target deploy.Target, j *Journal, err error) error {
			// Should see two deletes:  the default provider should be injected into the old state before the update
			// runs.
			deleted := make(map[resource.URN]bool)
			for _, entry := range j.Entries {
				switch urn := entry.Step.URN(); urn {
				case provURN, resURN:
					deleted[urn] = true
					assert.Equal(t, deploy.OpDelete, entry.Step.Op())
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			assert.Len(t, deleted, 2)
			assert.Len(t, j.Snap(target.Snapshot).Resources, 0)
			return err
		},
	}}
	p.Run(t, old)

	// Run a partial lifecycle using the base snapshot, skipping the initial update step.
	p.Steps = MakeBasicLifecycleSteps(t, 2)[1:]
	p.Run(t, old)
}

func TestSingleResourceDefaultProviderReplace(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(olds, news resource.PropertyMap) (plugin.DiffResult, error) {
					// Always require replacement.
					keys := []resource.PropertyKey{}
					for k := range news {
						keys = append(keys, k)
					}
					return plugin.DiffResult{ReplaceKeys: keys}, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, "", false, nil, "",
			resource.PropertyMap{})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{host: host},
		Config: config.Map{
			config.MustMakeKey("pkgA", "foo"): config.NewValue("bar"),
		},
	}

	// Build a basic lifecycle.
	steps := MakeBasicLifecycleSteps(t, 2)

	// Run the lifecycle through its no-op update+refresh.
	p.Steps = steps[:4]
	snap := p.Run(t, nil)

	// Change the config and run an update. We expect everything to require replacement.
	p.Config[config.MustMakeKey("pkgA", "foo")] = config.NewValue("baz")
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, j *Journal, err error) error {
			provURN := p.NewProviderURN("pkgA", "default", "")
			resURN := p.NewURN("pkgA:m:typA", "resA", "")

			// Look for replace steps on the provider and the resource.
			replacedProvider, replacedResource := false, false
			for _, entry := range j.Entries {
				if entry.Kind != JournalEntrySuccess || entry.Step.Op() != deploy.OpDeleteReplaced {
					continue
				}

				switch urn := entry.Step.URN(); urn {
				case provURN:
					replacedProvider = true
				case resURN:
					replacedResource = true
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			assert.True(t, replacedProvider)
			assert.True(t, replacedResource)

			return err
		},
	}}
	snap = p.Run(t, snap)

	// Resume the lifecycle with another no-op update.
	p.Steps = steps[2:]
	p.Run(t, snap)
}

func TestSingleResourceExplicitProviderReplace(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(olds, news resource.PropertyMap) (plugin.DiffResult, error) {
					// Always require replacement.
					keys := []resource.PropertyKey{}
					for k := range news {
						keys = append(keys, k)
					}
					return plugin.DiffResult{ReplaceKeys: keys}, nil
				},
			}, nil
		}),
	}

	providerInputs := resource.PropertyMap{
		resource.PropertyKey("foo"): resource.NewStringProperty("bar"),
	}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true, "",
			false, nil, "", providerInputs)
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, "", false, nil, provRef.String(),
			resource.PropertyMap{})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{host: host},
	}

	// Build a basic lifecycle.
	steps := MakeBasicLifecycleSteps(t, 2)

	// Run the lifecycle through its no-op update+refresh.
	p.Steps = steps[:4]
	snap := p.Run(t, nil)

	// Change the config and run an update. We expect everything to require replacement.
	providerInputs[resource.PropertyKey("foo")] = resource.NewStringProperty("baz")
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, j *Journal, err error) error {
			provURN := p.NewProviderURN("pkgA", "provA", "")
			resURN := p.NewURN("pkgA:m:typA", "resA", "")

			// Look for replace steps on the provider and the resource.
			replacedProvider, replacedResource := false, false
			for _, entry := range j.Entries {
				if entry.Kind != JournalEntrySuccess || entry.Step.Op() != deploy.OpDeleteReplaced {
					continue
				}

				switch urn := entry.Step.URN(); urn {
				case provURN:
					replacedProvider = true
				case resURN:
					replacedResource = true
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			assert.True(t, replacedProvider)
			assert.True(t, replacedResource)

			return err
		},
	}}
	snap = p.Run(t, snap)

	// Resume the lifecycle with another no-op update.
	p.Steps = steps[2:]
	p.Run(t, snap)
}

func TestSingleResourceExplicitProviderDeleteBeforeReplace(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(olds, news resource.PropertyMap) (plugin.DiffResult, error) {
					// Always require replacement.
					keys := []resource.PropertyKey{}
					for k := range news {
						keys = append(keys, k)
					}
					return plugin.DiffResult{ReplaceKeys: keys, DeleteBeforeReplace: true}, nil
				},
			}, nil
		}),
	}

	providerInputs := resource.PropertyMap{
		resource.PropertyKey("foo"): resource.NewStringProperty("bar"),
	}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true, "",
			false, nil, "", providerInputs)
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, "", false, nil, provRef.String(),
			resource.PropertyMap{})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{host: host},
	}

	// Build a basic lifecycle.
	steps := MakeBasicLifecycleSteps(t, 2)

	// Run the lifecycle through its no-op update+refresh.
	p.Steps = steps[:4]
	snap := p.Run(t, nil)

	// Change the config and run an update. We expect everything to require replacement.
	providerInputs[resource.PropertyKey("foo")] = resource.NewStringProperty("baz")
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, j *Journal, err error) error {
			provURN := p.NewProviderURN("pkgA", "provA", "")
			resURN := p.NewURN("pkgA:m:typA", "resA", "")

			// Look for replace steps on the provider and the resource.
			createdProvider, createdResource := false, false
			deletedProvider, deletedResource := false, false
			for _, entry := range j.Entries {
				if entry.Kind != JournalEntrySuccess {
					continue
				}

				switch urn := entry.Step.URN(); urn {
				case provURN:
					if entry.Step.Op() == deploy.OpDeleteReplaced {
						assert.False(t, createdProvider)
						assert.False(t, createdResource)
						assert.True(t, deletedResource)
						deletedProvider = true
					} else if entry.Step.Op() == deploy.OpCreateReplacement {
						assert.True(t, deletedProvider)
						assert.True(t, deletedResource)
						assert.False(t, createdResource)
						createdProvider = true
					}
				case resURN:
					if entry.Step.Op() == deploy.OpDeleteReplaced {
						assert.False(t, deletedProvider)
						assert.False(t, deletedResource)
						deletedResource = true
					} else if entry.Step.Op() == deploy.OpCreateReplacement {
						assert.True(t, deletedProvider)
						assert.True(t, deletedResource)
						assert.True(t, createdProvider)
						createdResource = true
					}
				default:
					t.Fatalf("unexpected resource %v", urn)
				}
			}
			assert.True(t, deletedProvider)
			assert.True(t, deletedResource)

			return err
		},
	}}
	snap = p.Run(t, snap)

	// Resume the lifecycle with another no-op update.
	p.Steps = steps[2:]
	p.Run(t, snap)
}

func TestDestroyWithPendingDelete(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	host := deploytest.NewPluginHost(nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{host: host},
	}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Create an old snapshot with two copies of a resource that share a URN: one that is pending deletion and one
	// that is not.
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "0",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
				Delete:  true,
			},
		},
	}

	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(_ workspace.Project, _ deploy.Target, j *Journal, err error) error {
			// Verify that we see a DeleteReplacement for the resource with ID 0 and a Delete for the resouce with
			// ID 1.
			deletedID0, deletedID1 := false, false
			for _, entry := range j.Entries {
				// Ignore non-terminal steps and steps that affect the injected default provider.
				if entry.Kind != JournalEntrySuccess || entry.Step.URN() != resURN ||
					(entry.Step.Op() != deploy.OpDelete && entry.Step.Op() != deploy.OpDeleteReplaced) {
					continue
				}

				switch id := entry.Step.Old().ID; id {
				case "0":
					assert.False(t, deletedID0)
					deletedID0 = true
				case "1":
					assert.False(t, deletedID1)
					deletedID1 = true
				default:
					assert.Fail(t, "unexpected resource ID %v", string(id))
				}
			}
			assert.True(t, deletedID0)
			assert.True(t, deletedID1)

			return err
		},
	}}
	p.Run(t, old)
}

func TestUpdateWithPendingDelete(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	host := deploytest.NewPluginHost(nil, nil, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{host: host},
	}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Create an old snapshot with two copies of a resource that share a URN: one that is pending deletion and one
	// that is not.
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "0",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
				Delete:  true,
			},
		},
	}

	p.Steps = []TestStep{{
		Op: Destroy,
		Validate: func(_ workspace.Project, _ deploy.Target, j *Journal, err error) error {
			// Verify that we see a DeleteReplacement for the resource with ID 0 and a Delete for the resouce with
			// ID 1.
			deletedID0, deletedID1 := false, false
			for _, entry := range j.Entries {
				// Ignore non-terminal steps and steps that affect the injected default provider.
				if entry.Kind != JournalEntrySuccess || entry.Step.URN() != resURN ||
					(entry.Step.Op() != deploy.OpDelete && entry.Step.Op() != deploy.OpDeleteReplaced) {
					continue
				}

				switch id := entry.Step.Old().ID; id {
				case "0":
					assert.False(t, deletedID0)
					deletedID0 = true
				case "1":
					assert.False(t, deletedID1)
					deletedID1 = true
				default:
					assert.Fail(t, "unexpected resource ID %v", string(id))
				}
			}
			assert.True(t, deletedID0)
			assert.True(t, deletedID1)

			return err
		},
	}}
	p.Run(t, old)
}

func TestParallelRefresh(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Create a program that registers four resources, each of which depends on the resource that immediately precedes
	// it.
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, "", false, nil, "",
			resource.PropertyMap{})
		assert.NoError(t, err)

		resB, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, "", false, []resource.URN{resA}, "",
			resource.PropertyMap{})
		assert.NoError(t, err)

		resC, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, "", false, []resource.URN{resB}, "",
			resource.PropertyMap{})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, "", false, []resource.URN{resC}, "",
			resource.PropertyMap{})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Parallel: 4, host: host},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 5)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.Equal(t, string(snap.Resources[2].URN.Name()), "resB")
	assert.Equal(t, string(snap.Resources[3].URN.Name()), "resC")
	assert.Equal(t, string(snap.Resources[4].URN.Name()), "resD")

	p.Steps = []TestStep{{Op: Refresh}}
	snap = p.Run(t, snap)

	assert.Len(t, snap.Resources, 5)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.Equal(t, string(snap.Resources[2].URN.Name()), "resB")
	assert.Equal(t, string(snap.Resources[3].URN.Name()), "resC")
	assert.Equal(t, string(snap.Resources[4].URN.Name()), "resD")
}

func TestExternalRefresh(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Our program reads a resource and exits.
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "resA-some-id", "", resource.PropertyMap{}, "")
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		return nil
	})
	host := deploytest.NewPluginHost(nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{host: host},
		Steps:   []TestStep{{Op: Update}},
	}

	// The read should place "resA" in the snapshot with the "External" bit set.
	snap := p.Run(t, nil)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.True(t, snap.Resources[1].External)

	p = &TestPlan{
		Options: UpdateOptions{host: host},
		Steps:   []TestStep{{Op: Refresh}},
	}

	snap = p.Run(t, snap)
	// A refresh should leave "resA" as it is in the snapshot. The External bit should still be set.
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.True(t, snap.Resources[1].External)
}

func TestRefreshInitFailure(t *testing.T) {
	//
	// Refresh will persist any initialization errors that are returned by `Read`. This provider
	// will error out or not based on the value of `refreshShouldFail`.
	//
	refreshShouldFail := false

	//
	// Set up test environment to use `readFailProvider` as the underlying resource provider.
	//
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(
					urn resource.URN, id resource.ID, props resource.PropertyMap,
				) (resource.PropertyMap, resource.Status, error) {
					if refreshShouldFail {
						err := &plugin.InitError{
							Reasons: []string{"Refresh reports continued to fail to initialize"},
						}
						return resource.PropertyMap{}, resource.StatusPartialFailure, err
					}
					return resource.PropertyMap{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, "", false, nil, "",
			resource.PropertyMap{})
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{host: host},
	}

	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	//
	// Create an old snapshot with a single initialization failure.
	//
	old := &deploy.Snapshot{
		Resources: []*resource.State{{
			Type:       resURN.Type(),
			URN:        resURN,
			Custom:     true,
			ID:         "0",
			Inputs:     resource.PropertyMap{},
			Outputs:    resource.PropertyMap{},
			InitErrors: []string{"Resource failed to initialize"},
		}},
	}

	//
	// Refresh DOES NOT fail, causing the initialization error to disappear.
	//
	p.Steps = []TestStep{{Op: Refresh}}
	snap := p.Run(t, old)

	for _, resource := range snap.Resources {
		switch urn := resource.URN; urn {
		case provURN:
			// break
		case resURN:
			assert.Equal(t, []string{}, resource.InitErrors)
		default:
			t.Fatalf("unexpected resource %v", urn)
		}
	}

	//
	// Refresh DOES fail, causing the new initialization error to appear.
	//
	refreshShouldFail = true
	p.Steps = []TestStep{{Op: Refresh}}
	snap = p.Run(t, old)
	for _, resource := range snap.Resources {
		switch urn := resource.URN; urn {
		case provURN:
			// break
		case resURN:
			assert.Equal(t, []string{"Refresh reports continued to fail to initialize"}, resource.InitErrors)
		default:
			t.Fatalf("unexpected resource %v", urn)
		}
	}
}
