"""Host maintenance with signed, expiring, single-use approvals."""

# Shared stdlib-only metadata helpers stay in the already protected native
# package file; bootstrap never imports an additional unvalidated module.
import ipaddress
import re


class ContainerIdentityError(ValueError):
    pass


def _require_identity(condition):
    if not condition:
        raise ContainerIdentityError('invalid container recreation identity')


def _keys(value, fields):
    _require_identity(type(value) is dict and set(value) == set(fields))


def _text(value, *, empty=False):
    _require_identity(type(value) is str and (empty or bool(value)) and len(value) <= 4096
                      and all(ord(character) >= 32 and ord(character) != 127 for character in value))
    return value


def _strings(values):
    _require_identity(type(values) is list and len(values) <= 256)
    result = [_text(value) for value in values]
    _require_identity(len(set(result)) == len(result))
    return sorted(result)


def _static_ipam(value):
    if value is None:
        return None
    _keys(value, ('ipv4_address', 'ipv6_address', 'link_local_ips'))
    result = {key: _text(value[key], empty=True) for key in ('ipv4_address', 'ipv6_address')}
    try:
        for key, version in (('ipv4_address', 4), ('ipv6_address', 6)):
            if result[key]:
                _require_identity(ipaddress.ip_address(result[key]).version == version)
        result['link_local_ips'] = _strings(value['link_local_ips'])
        for address in result['link_local_ips']:
            ipaddress.ip_address(address)
    except ValueError:
        raise ContainerIdentityError('invalid container recreation identity') from None
    return result


def canonical_recreation_identity(value):
    _keys(value, ('schema_version', 'name', 'compose', 'mounts', 'network_mode', 'links', 'networks'))
    _require_identity(type(value['schema_version']) is int and value['schema_version'] == 1)
    _require_identity(type(value['name']) is str and re.fullmatch(r'/[A-Za-z0-9][A-Za-z0-9_.-]*', value['name']))
    compose = value['compose']
    _keys(compose, ('project', 'service'))
    _require_identity((compose['project'] is None and compose['service'] is None)
                      or (type(compose['project']) is str and type(compose['service']) is str
                          and bool(compose['project']) and bool(compose['service'])))
    if compose['project'] is not None:
        _text(compose['project'])
        _text(compose['service'])
    _require_identity(type(value['mounts']) is list and len(value['mounts']) <= 256)
    mounts, destinations = [], set()
    for mount in value['mounts']:
        _keys(mount, ('type', 'source', 'volume_name', 'destination', 'rw', 'propagation'))
        _require_identity(mount['type'] in ('bind', 'volume', 'tmpfs') and type(mount['rw']) is bool)
        source = _text(mount['source'], empty=mount['type'] == 'tmpfs')
        destination = _text(mount['destination'])
        _require_identity(destination.startswith('/') and destination not in destinations)
        destinations.add(destination)
        _require_identity(mount['propagation'] in ('', 'private', 'rprivate', 'shared', 'rshared', 'slave', 'rslave'))
        if mount['type'] == 'volume':
            _text(mount['volume_name'])
        else:
            _require_identity(mount['volume_name'] is None)
        _require_identity(not source or source.startswith('/'))
        mounts.append(dict(mount))
    _require_identity(type(value['networks']) is list and len(value['networks']) <= 64)
    networks, names, identifiers = [], set(), set()
    for network in value['networks']:
        _keys(network, ('name', 'network_id', 'aliases', 'static_ipam', 'links', 'driver_options_present'))
        _require_identity(type(network['driver_options_present']) is bool)
        name = _text(network['name'])
        identifier = network['network_id']
        _require_identity(type(identifier) is str and re.fullmatch(r'[0-9a-f]{64}', identifier)
                          and name not in names and identifier not in identifiers)
        names.add(name)
        identifiers.add(identifier)
        networks.append({'name': name, 'network_id': identifier, 'aliases': _strings(network['aliases']),
                         'static_ipam': _static_ipam(network['static_ipam']), 'links': _strings(network['links']),
                         'driver_options_present': network['driver_options_present']})
    return {'schema_version': 1, 'name': value['name'], 'compose': dict(compose),
            'mounts': sorted(mounts, key=lambda item: item['destination']),
            'network_mode': _text(value['network_mode']), 'links': _strings(value['links']),
            'networks': sorted(networks, key=lambda item: item['name'])}


def normalize_recreation_identity(value, container_id):
    _keys(value, ('schema_version', 'name', 'compose', 'mounts', 'network_mode', 'links', 'networks'))
    _require_identity(type(container_id) is str and re.fullmatch(r'[0-9a-f]{64}', container_id))
    compose = value['compose']
    _keys(compose, ('project_present', 'project', 'service_present', 'service'))
    _require_identity(type(compose['project_present']) is bool and type(compose['service_present']) is bool
                      and compose['project_present'] == compose['service_present'])
    for key in ('project', 'service'):
        _text(compose[key], empty=not compose[key + '_present'])
        _require_identity(compose[key + '_present'] or compose[key] == '')
    selected = {**value, 'compose': {key: compose[key] if compose[key + '_present'] else None
                                   for key in ('project', 'service')},
                'links': [] if value['links'] is None else value['links']}
    _require_identity(type(value['networks']) is list)
    networks = []
    for network in value['networks']:
        _keys(network, ('name', 'network_id', 'aliases', 'static_ipam', 'links', 'driver_options_present'))
        aliases = [] if network['aliases'] is None else network['aliases']
        _require_identity(type(aliases) is list and len(aliases) <= 256)
        aliases = [_text(alias) for alias in aliases]
        # Compose can repeat container, service and configured DNS aliases.
        aliases = sorted(set(aliases))
        ipam = network['static_ipam']
        if ipam is not None:
            _keys(ipam, ('ipv4_address', 'ipv6_address', 'link_local_ips'))
            ipam = {**ipam, 'link_local_ips': [] if ipam['link_local_ips'] is None else ipam['link_local_ips']}
        networks.append({**network, 'aliases': [alias for alias in aliases if alias not in (container_id, container_id[:12])],
                         'links': [] if network['links'] is None else network['links'], 'static_ipam': ipam})
    selected['networks'] = networks
    return canonical_recreation_identity(selected)


def recreation_identity_matches(before, after):
    try:
        original, current = canonical_recreation_identity(before), canonical_recreation_identity(after)
        return (original == current and not original['network_mode'].startswith('container:')
                and not original['links'] and all(not network['links'] and not network['driver_options_present']
                                                 for network in original['networks']))
    except ContainerIdentityError:
        return False


# Only selected values are serialized. Presence flags distinguish missing
# Compose labels from explicitly empty labels without returning other labels.
CONTAINER_FORMAT = (
    '{{$projectPresent := false}}{{$project := ""}}{{$servicePresent := false}}{{$service := ""}}'
    '{{range $key, $value := .Config.Labels}}'
    '{{if eq $key "com.docker.compose.project"}}{{$projectPresent = true}}{{$project = $value}}{{end}}'
    '{{if eq $key "com.docker.compose.service"}}{{$servicePresent = true}}{{$service = $value}}{{end}}{{end}}'
    '{"id":{{json .Id}},"image_id":{{json .Image}},"image":{{json .Config.Image}},'
    '"status":{{json .State.Status}},"health":{{with index .State "Health"}}{{json .Status}}{{else}}"none"{{end}},'
    '"recreation_identity":{"schema_version":1,"name":{{json .Name}},'
    '"compose":{"project_present":{{$projectPresent}},"project":{{json $project}},'
    '"service_present":{{$servicePresent}},"service":{{json $service}}},'
    '"mounts":[{{$first := true}}{{range .Mounts}}{{if not $first}},{{end}}{{$first = false}}'
    '{"type":{{json .Type}},"source":{{json .Source}},'
    '"volume_name":{{if eq .Type "volume"}}{{json .Name}}{{else}}null{{end}},'
    '"destination":{{json .Destination}},"rw":{{json .RW}},"propagation":{{json .Propagation}}}{{end}}],'
    '"network_mode":{{json .HostConfig.NetworkMode}},"links":{{json .HostConfig.Links}},'
    '"networks":[{{$first := true}}{{range $name, $network := .NetworkSettings.Networks}}{{if not $first}},{{end}}{{$first = false}}'
    '{"name":{{json $name}},"network_id":{{json $network.NetworkID}},"aliases":{{json $network.Aliases}},'
    '"driver_options_present":{{if $network.DriverOpts}}true{{else}}false{{end}},'
    '"links":{{json $network.Links}},"static_ipam":{{with $network.IPAMConfig}}'
    '{"ipv4_address":{{json .IPv4Address}},"ipv6_address":{{json .IPv6Address}},"link_local_ips":{{json .LinkLocalIPs}}}'
    '{{else}}null{{end}}}{{end}}]}}')
