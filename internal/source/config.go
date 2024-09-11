package source

import (
	"errors"
	"fmt"

	"github.com/pentops/j5/gen/j5/config/v1/config_j5pb"
	"google.golang.org/protobuf/proto"
)

var ErrPluginCycle = errors.New("plugin cycle detected")

func resolveConfigReferences(config *config_j5pb.RepoConfigFile) error {

	overrides := map[string]*config_j5pb.PluginOverride{}
	for _, override := range config.PluginOverrides {
		overrides[override.Name] = override
	}
	rootPlugins := map[string]*config_j5pb.BuildPlugin{}

	for _, plugin := range config.Plugins {
		if plugin.Base == nil {
			rootPlugins[plugin.Name] = plugin
			continue
		}
		base, ok := rootPlugins[*plugin.Base]
		if !ok {
			didMatch := false
			for _, search := range config.Plugins {
				if search.Name == *plugin.Base {
					didMatch = true
					break
				}
			}
			if !didMatch {
				return fmt.Errorf("plugin %q extends base plugin %q which is not defined", plugin.Name, *plugin.Base)
			} else {
				return fmt.Errorf("plugin %q extends %q which is defined later in the source", plugin.Name, *plugin.Base)
			}
		}

		extended := extendPlugin(base, plugin)
		rootPlugins[plugin.Name] = extended
	}

	config.Plugins = nil

	resolvePlugins := func(plugins []*config_j5pb.BuildPlugin, baseOpts map[string]string) error {
		localBases := map[string]*config_j5pb.BuildPlugin{}
		for idx, plugin := range plugins {
			if plugin.Opts == nil {
				plugin.Opts = map[string]string{}
			}
			for k, v := range baseOpts {
				if _, ok := plugin.Opts[k]; !ok {
					plugin.Opts[k] = v
				}
			}

			if plugin.Base != nil {
				found, ok := rootPlugins[*plugin.Base]
				if !ok {
					found, ok = localBases[*plugin.Base]
					if !ok {
						return fmt.Errorf("plugin %q extends base plugin %q which is not defined", plugin.Name, *plugin.Base)
					}
				}
				plugin = extendPlugin(found, plugin)
			}

			if plugin.Type == config_j5pb.Plugin_UNSPECIFIED {
				if plugin.Base == nil {
					return fmt.Errorf("plugin %q has no type, did you mean to set 'base'?", plugin.Name)
				}
			}

			if plugin.Name != "" {
				localBases[plugin.Name] = plugin
			}

			// override AFTER using as a base.
			if override, ok := overrides[plugin.Name]; ok {
				plugin.Local = override.Local
				plugin.Docker = override.Docker
			}
			plugins[idx] = plugin
		}
		return nil
	}

	for _, gen := range config.Generate {
		if err := resolvePlugins(gen.Plugins, gen.Opts); err != nil {
			return err
		}
	}
	for _, pub := range config.Publish {
		if err := resolvePlugins(pub.Plugins, pub.Opts); err != nil {
			return err
		}
	}

	return nil
}

func extendPlugin(base, ext *config_j5pb.BuildPlugin) *config_j5pb.BuildPlugin {
	ext = proto.Clone(ext).(*config_j5pb.BuildPlugin)
	if ext.Name == "" {
		ext.Name = base.Name
	}

	if ext.Local == nil && ext.Docker == nil {
		ext.Local = base.Local
		ext.Docker = base.Docker
		// If either are set, the extension wins.
	}

	if ext.Type == config_j5pb.Plugin_UNSPECIFIED {
		ext.Type = base.Type
	}

	// MERGE options.
	if ext.Opts == nil {
		ext.Opts = map[string]string{}
	}
	for k, v := range base.Opts {
		if _, ok := ext.Opts[k]; !ok {
			ext.Opts[k] = v
		}
	}
	return ext
}
