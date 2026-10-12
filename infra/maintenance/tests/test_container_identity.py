"""Synthetic container metadata only; never production names, paths or IPs."""
import copy
import unittest

import maintenance


def raw_identity():
    return {'schema_version': 1, 'name': '/fixture-sentinel',
            'compose': {'project_present': False, 'project': '', 'service_present': False, 'service': ''},
            'mounts': [
                {'type': 'bind', 'source': '/srv/fixture/data', 'volume_name': None,
                 'destination': '/fixture/data', 'rw': True, 'propagation': 'rprivate'},
                {'type': 'volume', 'source': '/srv/fixture/volume', 'volume_name': 'fixture-volume',
                 'destination': '/fixture/volume', 'rw': False, 'propagation': ''}],
            'network_mode': 'fixture-network', 'links': None,
            'networks': [{'name': 'fixture-network', 'network_id': '1' * 64,
                          'driver_options_present': False,
                          'aliases': ['fixture-alias', 'a' * 64, 'a' * 12], 'links': None,
                          'static_ipam': {'ipv4_address': '192.0.2.10', 'ipv6_address': '', 'link_local_ips': None}}]}


class ContainerIdentityTests(unittest.TestCase):
    def setUp(self):
        self.assertTrue(hasattr(maintenance, 'normalize_recreation_identity'), 'container identity normalization missing')
        self.raw = raw_identity()
        self.identity = maintenance.normalize_recreation_identity(self.raw, 'a' * 64)

    def test_normalization_preserves_known_absence_and_removes_only_self_aliases(self):
        self.assertEqual(self.identity['compose'], {'project': None, 'service': None})
        self.assertEqual(self.identity['networks'][0]['aliases'], ['fixture-alias'])
        self.assertEqual(self.identity['networks'][0]['static_ipam']['link_local_ips'], [])
        self.assertEqual(self.raw, raw_identity())
        alternate = raw_identity()
        alternate['networks'][0]['aliases'] = ['fixture-alias', 'b' * 64, 'b' * 12]
        changed = maintenance.normalize_recreation_identity(alternate, 'b' * 64)
        self.assertTrue(maintenance.recreation_identity_matches(self.identity, changed))
        alternate['networks'][0]['aliases'].append('a' * 12)
        changed = maintenance.normalize_recreation_identity(alternate, 'b' * 64)
        self.assertFalse(maintenance.recreation_identity_matches(self.identity, changed))

    def test_compose_container_service_and_configured_alias_overlap_is_normalized(self):
        # Compose appends container name, service name and configured aliases.
        # All three can name the same endpoint without changing DNS identity.
        raw = raw_identity()
        raw['name'] = '/fixture-alias'
        raw['compose'] = {'project_present': True, 'project': 'fixture-project',
                          'service_present': True, 'service': 'fixture-alias'}
        raw['networks'][0]['aliases'] = ['fixture-alias'] * 3
        original = copy.deepcopy(raw)
        expected = copy.deepcopy(self.identity)
        expected['name'] = '/fixture-alias'
        expected['compose'] = {'project': 'fixture-project', 'service': 'fixture-alias'}
        normalized = maintenance.normalize_recreation_identity(raw, 'a' * 64)
        self.assertEqual(normalized, expected)
        self.assertTrue(maintenance.recreation_identity_matches(expected, normalized))
        self.assertEqual(raw, original)

    def test_raw_alias_overlap_and_duplicate_self_ids_preserve_order_independent_identity(self):
        raw = raw_identity()
        raw['networks'][0]['aliases'] = ['z-fixture', 'a' * 64, 'fixture-alias', 'a' * 12,
                                         'z-fixture', 'a' * 12, 'fixture-alias', 'a' * 64]
        original = copy.deepcopy(raw)
        expected = copy.deepcopy(self.identity)
        expected['networks'][0]['aliases'] = ['fixture-alias', 'z-fixture']
        self.assertEqual(maintenance.normalize_recreation_identity(raw, 'a' * 64), expected)
        reordered = copy.deepcopy(raw)
        reordered['networks'][0]['aliases'].reverse()
        self.assertEqual(maintenance.normalize_recreation_identity(reordered, 'a' * 64), expected)
        self.assertEqual(raw, original)

    def test_raw_aliases_validate_type_count_and_every_text_before_deduplication(self):
        for aliases in ('fixture-alias', ('fixture-alias',), {}, [None], [True], [1],
                        ['fixture-alias', []], ['fixture-alias', ''],
                        ['fixture-alias', 'bad\x00alias'], ['fixture-alias', 'bad\x7falias'],
                        ['fixture-alias', 'x' * 4097], ['fixture-alias'] * 257,
                        ['a' * 64] * 257):
            raw = raw_identity()
            raw['networks'][0]['aliases'] = aliases
            with self.subTest(aliases=aliases), self.assertRaises(ValueError):
                maintenance.normalize_recreation_identity(raw, 'a' * 64)
        for aliases, expected in ((None, []), ([], []), (['fixture-alias'] * 256, ['fixture-alias'])):
            raw = raw_identity()
            raw['networks'][0]['aliases'] = aliases
            with self.subTest(count=None if aliases is None else len(aliases)):
                self.assertEqual(maintenance.normalize_recreation_identity(raw, 'a' * 64)
                                 ['networks'][0]['aliases'], expected)

    def test_signed_canonical_aliases_remain_strictly_unique(self):
        duplicate = copy.deepcopy(self.identity)
        duplicate['networks'][0]['aliases'] = ['fixture-alias', 'fixture-alias']
        with self.assertRaises(ValueError):
            maintenance.canonical_recreation_identity(duplicate)
        self.assertFalse(maintenance.recreation_identity_matches(self.identity, duplicate))
        self.assertFalse(maintenance.recreation_identity_matches(duplicate, self.identity))

    def test_each_selected_identity_field_change_denies_recreation(self):
        for path, value in [
            (('name',), '/other-fixture'), (('compose', 'project'), 'fixture-project'),
            (('compose', 'service'), 'fixture-service'), (('network_mode',), 'other-network'),
            (('mounts', 0, 'type'), 'volume'), (('mounts', 0, 'source'), '/srv/other-fixture'),
            (('mounts', 0, 'volume_name'), 'other-volume'), (('mounts', 0, 'destination'), '/other-destination'),
            (('mounts', 0, 'rw'), False), (('mounts', 0, 'propagation'), 'rshared'),
            (('networks', 0, 'name'), 'other-network'), (('networks', 0, 'network_id'), '2' * 64),
            (('networks', 0, 'aliases'), ['other-alias']),
            (('networks', 0, 'driver_options_present'), True),
            (('networks', 0, 'static_ipam', 'ipv4_address'), '192.0.2.11'),
            (('networks', 0, 'static_ipam', 'ipv6_address'), '2001:db8::10'),
            (('networks', 0, 'static_ipam', 'link_local_ips'), ['169.254.1.10'])]:
            candidate = copy.deepcopy(self.identity)
            destination = candidate
            for key in path[:-1]:
                destination = destination[key]
            destination[path[-1]] = value
            with self.subTest(path=path):
                self.assertFalse(maintenance.recreation_identity_matches(self.identity, candidate))

    def test_complete_mount_and_network_sets_are_required_but_order_is_irrelevant(self):
        first = raw_identity()
        first['networks'].append({'name': 'fixture-second', 'network_id': '2' * 64,
                                  'driver_options_present': False,
                                  'aliases': ['second-alias'], 'static_ipam': None, 'links': None})
        identity = maintenance.normalize_recreation_identity(first, 'a' * 64)
        reordered = copy.deepcopy(first)
        reordered['mounts'].reverse()
        reordered['networks'].reverse()
        self.assertTrue(maintenance.recreation_identity_matches(
            identity, maintenance.normalize_recreation_identity(reordered, 'a' * 64)))
        for field in ('mounts', 'networks'):
            candidate = copy.deepcopy(identity)
            candidate[field].pop()
            with self.subTest(field=field):
                self.assertFalse(maintenance.recreation_identity_matches(identity, candidate))

    def test_duplicate_mount_destination_network_name_and_id_are_refused(self):
        for mutate in (
            lambda raw: raw['mounts'].append(copy.deepcopy(raw['mounts'][0])),
            lambda raw: raw['networks'].append(copy.deepcopy(raw['networks'][0])),
            lambda raw: raw['networks'].append({**copy.deepcopy(raw['networks'][0]), 'name': 'other-network'})):
            value = raw_identity()
            mutate(value)
            with self.assertRaises(ValueError):
                maintenance.normalize_recreation_identity(value, 'a' * 64)

    def test_duplicate_links_and_static_ipam_entries_are_still_refused(self):
        for path, values in [(('links',), ['/fixture-other:/fixture-alias'] * 2),
                             (('networks', 0, 'links'), ['/fixture-other:/fixture-alias'] * 2),
                             (('networks', 0, 'static_ipam', 'link_local_ips'), ['169.254.1.10'] * 2)]:
            raw = raw_identity()
            destination = raw
            for key in path[:-1]:
                destination = destination[key]
            destination[path[-1]] = values
            with self.subTest(path=path), self.assertRaises(ValueError):
                maintenance.normalize_recreation_identity(raw, 'a' * 64)

    def test_compose_absence_is_distinct_from_present_empty_and_partial_labels(self):
        for compose in ({'project_present': True, 'project': '', 'service_present': True, 'service': ''},
                        {'project_present': True, 'project': 'fixture', 'service_present': False, 'service': ''},
                        {'project_present': False, 'project': 'fixture', 'service_present': False, 'service': ''},
                        {'project_present': 0, 'project': '', 'service_present': False, 'service': ''},
                        {'project': None, 'service': None}):
            raw = raw_identity()
            raw['compose'] = compose
            with self.subTest(compose=compose), self.assertRaises(ValueError):
                maintenance.normalize_recreation_identity(raw, 'a' * 64)
        raw = raw_identity()
        raw['compose'] = {'project_present': True, 'project': 'fixture', 'service_present': True, 'service': 'sentinel'}
        self.assertEqual(maintenance.normalize_recreation_identity(raw, 'a' * 64)['compose'],
                         {'project': 'fixture', 'service': 'sentinel'})

    def test_missing_unknown_schema_and_bad_selected_types_are_refused(self):
        for raw in (None, {}, {**self.raw, 'schema_version': True}, {**self.raw, 'schema_version': 2},
                    {**self.raw, 'unknown': 'fixture'}, {**self.raw, 'mounts': None},
                    {**self.raw, 'network_mode': None}):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                maintenance.normalize_recreation_identity(raw, 'a' * 64)
        for path, value in [(('mounts', 0, 'rw'), 1), (('mounts', 0, 'source'), None),
                            (('networks', 0, 'network_id'), ''), (('networks', 0, 'aliases'), 'alias'),
                            (('networks', 0, 'static_ipam', 'ipv4_address'), None),
                            (('networks', 0, 'static_ipam', 'link_local_ips'), [True])]:
            raw = raw_identity()
            destination = raw
            for key in path[:-1]:
                destination = destination[key]
            destination[path[-1]] = value
            with self.subTest(path=path), self.assertRaises(ValueError):
                maintenance.normalize_recreation_identity(raw, 'a' * 64)
        self.assertFalse(maintenance.recreation_identity_matches(None, self.identity))
        self.assertFalse(maintenance.recreation_identity_matches({}, self.identity))

    def test_container_mode_and_host_or_endpoint_links_cannot_authorize_recreation(self):
        for path, value in [(('network_mode',), 'container:' + 'f' * 64),
                            (('links',), ['/fixture-other:/fixture-alias']),
                            (('networks', 0, 'links'), ['/fixture-other:/fixture-alias'])]:
            raw = raw_identity()
            target = raw
            for key in path[:-1]:
                target = target[key]
            target[path[-1]] = value
            selected = maintenance.normalize_recreation_identity(raw, 'a' * 64)
            with self.subTest(path=path):
                self.assertFalse(maintenance.recreation_identity_matches(selected, selected))

    def test_endpoint_driver_options_presence_is_required_and_custom_options_deny_recreation(self):
        missing = copy.deepcopy(self.identity)
        missing['networks'][0].pop('driver_options_present', None)
        self.assertFalse(maintenance.recreation_identity_matches(missing, missing))
        for value in (None, 0, 1, '', {}, []):
            raw = raw_identity()
            raw['networks'][0]['driver_options_present'] = value
            with self.subTest(value=value), self.assertRaises(ValueError):
                maintenance.normalize_recreation_identity(raw, 'a' * 64)
        raw = raw_identity()
        raw['networks'][0]['driver_options_present'] = True
        selected = maintenance.normalize_recreation_identity(raw, 'a' * 64)
        self.assertFalse(maintenance.recreation_identity_matches(selected, selected))
        self.assertFalse(maintenance.recreation_identity_matches(self.identity, selected))
        self.assertFalse(maintenance.recreation_identity_matches(selected, self.identity))

    def test_driver_option_values_are_never_selected_or_reported(self):
        projection = maintenance.CONTAINER_FORMAT
        self.assertIn('"driver_options_present":{{if $network.DriverOpts}}true{{else}}false{{end}}', projection)
        self.assertNotIn('{{json $network.DriverOpts}}', projection)
        self.assertNotIn('range $network.DriverOpts', projection)
        raw = raw_identity()
        raw['networks'][0]['driver_options_present'] = True
        raw['networks'][0]['driver_options'] = {'fixture-private-option': 'fixture-private-value'}
        with self.assertRaises(ValueError) as raised:
            maintenance.normalize_recreation_identity(raw, 'a' * 64)
        self.assertNotIn('fixture-private', str(raised.exception))

    def test_inspect_projection_selects_metadata_and_excludes_private_and_dynamic_fields(self):
        projection = maintenance.CONTAINER_FORMAT
        for field in ('.Name', 'com.docker.compose.project', 'com.docker.compose.service',
                      '.Mounts', '.Source', '.Destination', '.RW', '.Propagation',
                      '.HostConfig.NetworkMode', '.HostConfig.Links', '.NetworkSettings.Networks',
                      '.NetworkID', '.Aliases', '.IPAMConfig', '.IPv4Address', '.IPv6Address', '.LinkLocalIPs'):
            self.assertIn(field, projection)
        for forbidden in ('.Config.Env', '{{json .Config}}', '{{json .HostConfig}}', '{{json .Config.Labels}}',
                          '{{json .NetworkSettings}}', '.IPAddress', '.Gateway', '.MacAddress',
                          '.EndpointID', '.DNSNames'):
            self.assertNotIn(forbidden, projection)


if __name__ == '__main__':
    unittest.main()
