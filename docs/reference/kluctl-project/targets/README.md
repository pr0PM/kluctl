<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "targets"
linkTitle: "targets"
weight: 4
description: >
  Required, defines targets for this kluctl project.
---
-->

# targets

Specifies a list of targets for which commands can be invoked. A target puts together environment/target specific
configuration and the target cluster. Multiple targets can exist which target the same cluster but with differing
configuration (via `args`). Target entries also specifies which secrets to use while [sealing](../../sealed-secrets.md).

Each value found in the target definition is rendered with a simple Jinja2 context that only contains the target itself.
The rendering process is retried 10 times until it finally succeeds, allowing you to reference
the target itself in complex ways. This is especially useful when using [dynamic targets](./dynamic-targets.md).

Target entries have the following form:
```yaml
targets:
...
  - name: <target_name>
    context: <context_name>
    args:
      arg1: <value1>
      arg2: <value2>
      ...
    images:
      - image: my-image
        resultImage: my-image:1.2.3
    sealingConfig:
      secretSets:
        - <name_of_secrets_set>
...
```

The following fields are allowed per target:

## name
This field specifies the name of the target. The name must be unique. It is referred in all commands via the
[-t](../../commands/common-arguments.md) option.

## context
This field specifies the kubectl context of the target cluster. The context must exist in the currently active kubeconfig.
If this field is omitted, Kluctl will always use the currently active context.

## args
This fields specifies a map of arguments to be passed to the deployment project when it is rendered. Allowed argument names
are configured via [deployment args](../../deployments/deployment-yml.md#args).

The arguments specified in the [dynamic target config](../../kluctl-project/targets/dynamic-targets.md#args)
have higher priority.

## images
This field specifies a list of fixed images to be used by [`images.get_image(...)`](../../deployments/images.md#imagesget_image).
The format is identical to the [fixed images file](../../deployments/images.md#fixed-images-via-a-yaml-file).

The fixed images specified in the [dynamic target config](../../kluctl-project/targets/dynamic-targets.md#images)
have higher priority.

## sealingConfig
This field configures how sealing is performed when the [seal command](../../commands/seal.md) is invoked for this target.
It has the following form:

```yaml
targets:
...
- name: <target_name>
  ...
  sealingConfig:
    args:
      arg1: <override_for_arg1>
    certFile: <path-to-cert-file>
    dynamicSealing: <true_or_false>
    secretSets:
      - <name_of_secrets_set>
```

### args
This field allows adding extra arguments to the target args. These are only used while sealing and may override
arguments which are already configured for the target.

### certFile
Optional path to a local (inside your project) public certificate used for sealing. Such a certificate can be fetched
from the sealed-secrets controller using `kubeseal --fetch-cert`.

### dynamicSealing
This field specifies weather sealing should happen per [dynamic target](./dynamic-targets.md) or only once. This
field is optional and defaults to `true`.

### secretSets
This field specifies a list of secret set names, which all must exist in the [secretsConfig](../secrets-config).
