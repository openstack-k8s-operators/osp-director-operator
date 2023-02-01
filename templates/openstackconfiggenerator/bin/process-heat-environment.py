#!/usr/bin/env python3

import argparse
import collections
import yaml


def process_environment_file(filename):
    with open(filename, 'r', encoding='utf-8') as orig:
        original_data = orig.read()
        original_env = yaml.safe_load(original_data)

    if not isinstance(original_env, collections.abc.Mapping) or \
            'parameter_defaults' not in original_env:
        # Empty environment file or no parameter defaults? Do nothing
        return

    new_env = dict(original_env)
    for key, val in new_env.get('parameter_defaults', {}).items():
        if isinstance(val, collections.abc.Mapping):
            new_env.setdefault(
                'parameter_merge_strategies', {}
            ).setdefault(key, 'merge')

    if (new_env.get('parameter_merge_strategies') !=
            original_env.get('parameter_merge_strategies')):
        with open(filename, 'w', encoding='utf-8') as new:
            new.write(yaml.dump(new_env, default_flow_style=False))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(
        description='Process heat environment files to set merge strategy for '
                    'all mapping parameters'
    )
    parser.add_argument(
        '-e', '--environment',
        metavar='<environment>',
        action='append',
        required=True,
        help='Path to the environment. Can be specified multiple times'
    )
    args = parser.parse_args()
    for f in args.environment:
        process_environment_file(f)
