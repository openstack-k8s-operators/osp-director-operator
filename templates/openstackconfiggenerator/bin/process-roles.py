#! /usr/bin/env python3

import argparse
import collections
import logging
import sys
import unittest
import unittest.mock
import yaml

logging.basicConfig(stream=sys.stdout, level=logging.INFO)
LOG = logging.getLogger('process-roles')


class TemplateLoader(yaml.SafeLoader):
    def construct_mapping(self, node):
        self.flatten_mapping(node)
        return collections.OrderedDict(self.construct_pairs(node))


class TemplateDumper(yaml.SafeDumper):
    def represent_ordered_dict(self, data):
        return self.represent_dict(data.items())

    def description_presenter(self, data):
        if not len(data) > 80:
            return self.represent_scalar('tag:yaml.org,2002:str', data)
        return self.represent_scalar('tag:yaml.org,2002:str', data, style='>')


TemplateDumper.add_representer(str,
                               TemplateDumper.description_presenter)
TemplateDumper.add_representer(collections.OrderedDict,
                               TemplateDumper.represent_ordered_dict)
TemplateLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG,
                               TemplateLoader.construct_mapping)


def get_role_counts(environment):
    """Find role counts in heat environment files"""
    role_counts = {}
    for env_file in environment:
        try:
            with open(env_file, 'r', encoding='utf-8') as envfile:
                data = yaml.safe_load(envfile.read())
            if data is not None:
                for param, val in data.get('parameter_defaults', {}).items():
                    if param.endswith('Count'):
                        role_name = param[:-5]
                        try:
                            role_count = int(val)
                        except ValueError:
                            continue
                        role_counts[role_name] = role_count
                        LOG.info(
                            "Found role count param '%s: %d' "
                            "in environment file '%s'",
                            role_name,
                            role_count,
                            env_file
                        )
        except Exception:
            LOG.exception("Failed to parse environment file %s", env_file)
            continue
    return role_counts


def filter_roles(roles_file, environment):
    """Read role count params from environment files and
       remove unused roles from roles_files"""
    role_counts = get_role_counts(environment)

    LOG.info("Loading original role data from '%s'", roles_file)
    with open(roles_file, 'r', encoding='utf-8') as orig:
        original_data = orig.read()
        original_roles = yaml.load(original_data, Loader=TemplateLoader)

    # Omit roles that are not in use or count is 0
    new_roles = []
    for role in original_roles:
        if role_counts.get(role['name'], 0) > 0:
            LOG.info("Including active role '%s'", role['name'])
            new_roles.append(role)
        else:
            LOG.warning("Excluding inactive role '%s'", role['name'])

    LOG.info("Updating role data in '%s'", roles_file)
    with open(roles_file, 'w', encoding='utf-8') as new:
        new.write(
            yaml.dump(
                new_roles,
                Dumper=TemplateDumper,
                default_flow_style=False
            )
        )


class ProcessRolesTest(unittest.TestCase):

    def test_get_role_counts_basic(self):
        with unittest.mock.patch('builtins.open', unittest.mock.mock_open(
            read_data='''
                parameter_defaults:
                  Foo: bar
                  ThatRoleCount: 234
                  FooACCount: me
                  ThisRoleCount: 7
                  Bar: baz
            '''
        )):
            role_counts = get_role_counts(['foo'])
        self.assertEqual(role_counts, {"ThatRole": 234, "ThisRole": 7})

    def test_filter_roles(self):
        with unittest.mock.patch(
                    self.__module__+'.get_role_counts'
                ) as m_get_role_counts:
            m_get_role_counts.return_value = {"ThatRole": 234, "ThisRole": 7}

            m_open_output_file = unittest.mock.mock_open()

            with unittest.mock.patch('builtins.open', unittest.mock.mock_open(
                read_data='''
                    - name: ThisRole
                      description: A role to include
                    - name: Notme
                      description: A role to exclude
                      CountDefault: 12
                    - name: NotThisRole
                      description: Exclude me too
                    - name: ThatRole
                      description: Include this one also
                '''
            )) as m_open_input_file:
                m_open_input_file.side_effect = (
                    m_open_input_file.return_value,
                    m_open_output_file.return_value
                )
                filter_roles('foobar.yaml', ['foo'])

            result = yaml.load(
                m_open_output_file().write.call_args_list[0].args[0],
                Loader=TemplateLoader
            )
            included_roles = [x['name'] for x in result]
            self.assertEqual(included_roles, ["ThisRole", "ThatRole"])


if __name__ == '__main__':
    parser = argparse.ArgumentParser(
        description='Filter unused roles in role_data'
    )
    parser.add_argument(
        '-r', '--roles-data',
        metavar='<roles_data>',
        required=True,
        help='Path to the roles data'
    )
    parser.add_argument(
        '-e', '--environment',
        metavar='<environment>',
        action='append',
        required=True,
        help='Path to the environment. Can be specified multiple times'
    )
    args = parser.parse_args()
    filter_roles(args.roles_data, args.environment)
