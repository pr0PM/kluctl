package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/git/auth"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/git/repocache"
	ssh_pool "github.com/kluctl/kluctl/v2/pkg/git/ssh-pool"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/registries"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"os"
	"strings"
)

func withKluctlProjectFromArgs(projectFlags args.ProjectFlags, strictTemplates bool, forCompletion bool, cb func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error) error {
	tmpDir, err := os.MkdirTemp(utils.GetTmpBaseDir(), "project-")
	if err != nil {
		return fmt.Errorf("creating temporary project directory failed: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	j2, err := kluctl_jinja2.NewKluctlJinja2(strictTemplates)
	if err != nil {
		return err
	}
	defer j2.Close()

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repoRoot, err := git.DetectGitRepositoryRoot(cwd)
	if err != nil {
		status.Warning(cliCtx, "Failed to detect git project root. This might cause follow-up errors")
	}

	ctx, cancel := context.WithTimeout(cliCtx, projectFlags.Timeout)
	defer cancel()

	sshPool := &ssh_pool.SshPool{}

	var repoOverrides []repocache.RepoOverride
	for _, x := range projectFlags.LocalGitOverride {
		ro, err := parseRepoOverride(x)
		if err != nil {
			return err
		}
		repoOverrides = append(repoOverrides, ro)
	}

	rp := repocache.NewGitRepoCache(ctx, sshPool, auth.NewDefaultAuthProviders(), repoOverrides, projectFlags.GitCacheUpdateInterval)
	defer rp.Clear()

	loadArgs := kluctl_project.LoadKluctlProjectArgs{
		RepoRoot:           repoRoot,
		ProjectDir:         cwd,
		ProjectConfig:      projectFlags.ProjectConfig.String(),
		RP:                 rp,
		ClientConfigGetter: clientConfigGetter(forCompletion),
	}

	p, err := kluctl_project.LoadKluctlProject(ctx, loadArgs, tmpDir, j2)
	if err != nil {
		return err
	}

	return cb(ctx, p)
}

type projectTargetCommandArgs struct {
	projectFlags         args.ProjectFlags
	targetFlags          args.TargetFlags
	argsFlags            args.ArgsFlags
	imageFlags           args.ImageFlags
	inclusionFlags       args.InclusionFlags
	dryRunArgs           *args.DryRunFlags
	renderOutputDirFlags args.RenderOutputDirFlags

	forSeal           bool
	forCompletion     bool
	offlineKubernetes bool
}

type commandCtx struct {
	ctx       context.Context
	targetCtx *kluctl_project.TargetContext
	images    *deployment.Images
}

func withProjectCommandContext(args projectTargetCommandArgs, cb func(ctx *commandCtx) error) error {
	return withKluctlProjectFromArgs(args.projectFlags, true, false, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
		return withProjectTargetCommandContext(ctx, args, p, cb)
	})
}

func withProjectTargetCommandContext(ctx context.Context, args projectTargetCommandArgs, p *kluctl_project.LoadedKluctlProject, cb func(ctx *commandCtx) error) error {
	rh := registries.NewRegistryHelper(ctx)
	err := rh.ParseAuthEntriesFromEnv()
	if err != nil {
		return fmt.Errorf("failed to parse registry auth from environment: %w", err)
	}
	images, err := deployment.NewImages(rh, args.imageFlags.UpdateImages, args.imageFlags.OfflineImages || args.forCompletion)
	if err != nil {
		return err
	}
	fixedImages, err := args.imageFlags.LoadFixedImagesFromArgs()
	if err != nil {
		return err
	}
	images.PrependFixedImages(fixedImages)

	inclusion, err := args.inclusionFlags.ParseInclusionFromArgs()
	if err != nil {
		return err
	}

	optionArgs, err := deployment.ParseArgs(args.argsFlags.Arg)
	if err != nil {
		return err
	}
	optionArgs2, err := deployment.ConvertArgsToVars(optionArgs, true)
	if err != nil {
		return err
	}
	for _, a := range args.argsFlags.ArgsFromFile {
		optionArgs3, err := uo.FromFile(a)
		if err != nil {
			return err
		}
		optionArgs2.Merge(optionArgs3)
	}

	renderOutputDir := args.renderOutputDirFlags.RenderOutputDir
	if renderOutputDir == "" {
		tmpDir, err := os.MkdirTemp(p.TmpDir, "rendered")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		renderOutputDir = tmpDir
	}

	targetParams := kluctl_project.TargetContextParams{
		TargetName:         args.targetFlags.Target,
		TargetNameOverride: args.targetFlags.TargetNameOverride,
		ContextOverride:    args.targetFlags.Context,
		OfflineK8s:         args.offlineKubernetes,
		DryRun:             args.dryRunArgs == nil || args.dryRunArgs.DryRun || args.forCompletion,
		ExternalArgs:       optionArgs2,
		ForSeal:            args.forSeal,
		Images:             images,
		Inclusion:          inclusion,
		RenderOutputDir:    renderOutputDir,
	}

	targetCtx, err := p.NewTargetContext(ctx, targetParams)
	if err != nil {
		return err
	}

	if !args.forSeal && !args.forCompletion {
		err = targetCtx.DeploymentCollection.Prepare()
		if err != nil {
			return err
		}
	}

	cmdCtx := &commandCtx{
		ctx:       ctx,
		targetCtx: targetCtx,
		images:    images,
	}

	return cb(cmdCtx)
}

func clientConfigGetter(forCompletion bool) func(context *string) (*rest.Config, *api.Config, error) {
	return func(context *string) (*rest.Config, *api.Config, error) {
		if forCompletion {
			return nil, nil, nil
		}

		configLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		if context != nil {
			configOverrides.CurrentContext = *context
		}
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides)
		rawConfig, err := clientConfig.RawConfig()
		if err != nil {
			return nil, nil, err
		}
		if context != nil {
			rawConfig.CurrentContext = *context
		}
		restConfig, err := clientConfig.ClientConfig()
		if err != nil {
			return nil, nil, err
		}
		return restConfig, &rawConfig, nil
	}
}

func parseRepoOverride(s string) (ret repocache.RepoOverride, err error) {
	sp := strings.SplitN(s, "=", 2)
	if len(sp) != 2 {
		return repocache.RepoOverride{}, fmt.Errorf("invalid --local-git-override %s", s)
	}

	sp2 := strings.Split(sp[0], ":")
	if len(sp2) < 2 || len(sp2) > 3 {
		return repocache.RepoOverride{}, fmt.Errorf("invalid --local-git-override %s", s)
	}

	u, err := git_url.Parse(fmt.Sprintf("%s:%s", sp2[0], sp2[1]))
	if err != nil {
		return repocache.RepoOverride{}, fmt.Errorf("invalid --local-git-override %s: %w", s, err)
	}

	ret.RepoKey = u.NormalizedRepoKey()
	if len(sp2) == 3 {
		ret.Ref = sp2[2]
	}
	ret.Override = sp[1]
	return
}
