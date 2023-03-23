#!/usr/bin/env python3
"""
Connection tool for nodes and databases, using the PMM
inventory as the source for the DSNs
"""
# pylint: disable=redefined-outer-name
from argparse import (
    ArgumentDefaultsHelpFormatter,
    ArgumentParser,
    FileType,
)
import base64
import binascii
from collections import namedtuple
import json
import logging
import netrc
import os
import shlex
import ssl
import subprocess
import sys
import syslog
from typing import (
    Dict,
    Optional,
)
import urllib.error
import urllib.parse
import urllib.request

LOGGING_CHOICES = (
    'debug', 'info', 'warning', 'error', 'critical'
)

DEFAULT_CONFIG_PATH = os.path.join(os.getenv('HOME'), '.config', 'gascan', 'connect-py.json')
DEFAULT_CONNECT_TIMEOUT = 120
DEFAULT_LOGGING_CHOICE = 'error'
DEFAULT_NETRC_PATH = os.path.join(os.getenv('HOME'), '.netrc')
DEFAULT_SERVER_ADDRESS = 'https://localhost:8443'

SAMPLE_CONFIG = dict(
    log_level="warning",
    server_address=DEFAULT_SERVER_ADDRESS,
    tls_insecure=False,
    inventory=dict(
        source="pmm",
        hosts=dict(
            testme=dict(
                ssh=dict(
                    custom_labels = dict(
                        port=22
                    ),
                )
            ),
            testmetoo=dict(
                mysql=dict(
                    port=33060
                )
            )
        ),
    ),
)


class FatalError(UserWarning):
    """Fatal error"""


class RequestError(UserWarning):
    """Request error"""


def parse_args() -> dict:
    """Runtime configuration"""
    parser = ArgumentParser(formatter_class=ArgumentDefaultsHelpFormatter)
    parser.add_argument('--server-address', default=DEFAULT_SERVER_ADDRESS,
                        help='The address for PMM Server')
    parser.add_argument('--tls-insecure', action='store_true',
                        help='Disable TLS security checks')
    parser.add_argument('--force', action='store_true',
                        help='Force requests to PMM')
    parser.add_argument('--connect-timeout', type=int, default=DEFAULT_CONNECT_TIMEOUT,
                        help='Timeout seconds for a connection')
    parser.add_argument('--list', action='store_true',
                        help='List all connections')
    parser.add_argument('--standardise', action='store_true',
                        help='Standardise naming of connections to use the node name')
    parser.add_argument('--log-level', choices=LOGGING_CHOICES,
                        default=DEFAULT_LOGGING_CHOICE,
                        help='Logging verbosity')
    parser.add_argument('--netrc-file', default=DEFAULT_NETRC_PATH,
                        type=FileType('rb'), help='Use an alternative netrc file')
    parser.add_argument('--config', default=DEFAULT_CONFIG_PATH,
                        type=FileType('rb'), help='Load overrides from a config file')
    parser.add_argument('--sample-config', action='store_true',
                        help='Generate a sample config')

    parser.add_argument('connection_options', nargs='*', default=[])
    return vars(parser.parse_args())


def log() -> logging.Logger:
    """Get a log handler"""
    return logging.getLogger(__name__)


class Connect: # pylint: disable=too-many-instance-attributes
    """Generate an instance connection"""
    _force_requests = False
    _standardised_naming = False

    _default_commands = dict(
        mongodb="/usr/bin/mongo",
        mysql="/usr/bin/mysql",
        postgresql="/usr/bin/psql",
        ssh="/usr/bin/ssh",
    )
    _default_ports = dict(
        mongodb=27011,
        mysql=3306,
        postgresql=1234,
        ssh=22,
    )

    _nodes = {}
    _overrides = {}
    _services = {}

    __user = os.getenv('SUDO_USER', os.getenv('USER', 'unknown'))

    def __init__(self, uri: str, verify_tls: bool, netrc_path: str, overrides: Optional[Dict] = None):
        self.base_uri = uri
        self.uri = urllib.parse.urlparse(self.base_uri)

        if isinstance(overrides, dict):
            self._overrides = overrides
            self._default_commands.update(self._overrides.get('default_commands', {}))
            self._default_ports.update(self._overrides.get('default_ports', {}))

        credentials = netrc.netrc(netrc_path)
        auth = credentials.authenticators(self.uri.hostname)

        if not auth or len(auth) != 3:
            raise FatalError('No auth found')

        password_mgr = urllib.request.HTTPPasswordMgrWithDefaultRealm()
        if b'id' not in base64.b64decode(auth[2]):
            raise FatalError('Unexpected content in token')
        password_mgr.add_password(realm=None, uri=self.base_uri, user='api_key', passwd=auth[2])

        auth_handler = urllib.request.HTTPBasicAuthHandler(password_mgr)

        if not verify_tls:
            tls_context = ssl._create_unverified_context()
        else:
            tls_context = ssl.create_default_context()
        tls_handler = urllib.request.HTTPSHandler(context=tls_context)

        opener = urllib.request.build_opener(auth_handler, tls_handler)
        urllib.request.install_opener(opener)
        self._headers = {'content-type': 'application/json',
                         'Authorization': f'Bearer {password_mgr.find_user_password(None, self.base_uri)[1]}'}

    @property
    def force_requests(self) -> bool:
        """Flag to force requests"""
        return self._force_requests

    @force_requests.setter
    def force_requests(self, val: bool):
        """Set flag for forcing requests"""
        self._force_requests = bool(val)

    @property
    def nodes(self) -> dict:
        """Lookup all nodes"""
        if not self._nodes:
            try:
                req = self._request(f'{self.base_uri}/v1/inventory/Nodes/List', payload=b'{}')
            except RequestError as err:
                log().error('Failed to connect to PMM at "%s"', self.base_uri)
                raise ConnectionError('Unable to proceed') from err

            nodes = {}
            Node = namedtuple('Node', ['id', 'name', 'type', 'address', 'distro', 'port', 'environment'])
            for node_type, data in req.get('json', {}).items():
                for node in data:
                    if self._overrides.get('hosts', {}).get(node['node_name']):
                        if 'ssh' in self._overrides['hosts'][node['node_name']]:
                            log().debug('Applying override for node %s: %s', node['node_name'],
                                        list(self._overrides['hosts'][node['node_name']]))
                            node.update(self._overrides['hosts'][node['node_name']]['ssh'])
                        # TODO: decide if this is needed
                        #if 'all' in self._overrides['hosts'][node['node_name']]:
                        #    log().debug('Applying override for node %s: %s', node['node_name'],
                        #                list(self._overrides['hosts'][node['node_name']]))
                        #    node.update(self._overrides['hosts'][node['node_name']]['all'])
                    nodes[node['node_id']] = Node(id=node['node_id'], name=node['node_name'], type=node_type,
                                                  address=node['address'], distro=node.get('distro'),
                                                  port=node.get('custom_labels', {}).get('port',
                                                                                         self._default_ports['ssh']),
                                                  environment=node.get('custom_labels', {}).get('environment'))
            if not self.force_requests:
                self._nodes = nodes
        return self._nodes

    @property
    def services(self) -> dict:
        """Lookup all services"""
        if not self._services:
            try:
                req = self._request(f'{self.base_uri}/v1/inventory/Services/List', payload=b'{}')
            except RequestError as err:
                log().error('Failed to connect to PMM at "%s"', self.base_uri)
                raise ConnectionError('Unable to proceed') from err

            services = {}
            Service = namedtuple('Service', ['cluster', 'id', 'name', 'node', 'port', 'type',
                                             'address', 'connect_cmd'])
            for service_type, data in req.get('json', {}).items():
                for service in data:
                    service_node = None
                    for node_id, node in self.nodes.items():
                        if node_id != service['node_id']:
                            continue
                        service_node = node
                        if ('hosts' not in self._overrides or
                            node.name not in self._overrides['hosts'] or
                            service_type not in self._overrides['hosts'][node.name]
                        ):
                            continue
                        service.update(self._overrides['hosts'][node.name][service_type])
                    services[service['service_id']] = Service(cluster=service.get('cluster'), id=service['service_id'],
                                                              name=service['service_name'],
                                                              port=service.get('port',
                                                                               self._default_ports[service_type]),
                                                              type=service_type, node=service_node,
                                                              address=service.get('address'),
                                                              connect_cmd=service.get('connect_cmd'))
            if not self.force_requests:
                self._services = services
        return self._services

    @property
    def standardise(self):
        """Standardise naming"""
        return self._standardised_naming

    @standardise.setter
    def standardise(self, val: bool):
        """Configure standardised naming"""
        self._standardised_naming = val

    @property
    def username(self):
        """Return the active username"""
        return self.__user

    def _request(self, uri: str, payload: str = '', timeout: int = 0):
        """Make a request"""
        try:
            args = {} if not payload else dict(data=payload)
            if timeout:
                args['timeout'] = timeout

            req = urllib.request.Request(uri, payload, self._headers, method='GET' if not payload else 'POST')
            with urllib.request.urlopen(req, **args) as resp:
                data = resp.read().decode('utf-8')
                return dict(raw=data, json=json.loads(data))
        except json.JSONDecodeError as err:
            raise RequestError('Unable to decode response') from err
        except urllib.error.HTTPError as err:
            raise RequestError('Unable to make request') from err

    def dbc(self, target, action='list', extra_args=None) -> list:
        """Connect via database client"""
        if action == 'connect':
            if not target:
                raise FatalError(f'Unable to use target={target} for database connections')

        def _cmd(service):
            try:
                connect_cmd = service.connect_cmd if service.connect_cmd else self._default_commands[service.type]
            except AttributeError:
                connect_cmd = self._default_commands[service.type]
            if service.type == 'mysql':
                return (f'{connect_cmd} --host={service.node.address} --port={service.port} '
                        f'--skip-auto-rehash --comments --safe-updates --connect-timeout=10 '
                        rf'--prompt="{service.node.name} \d> " ')
            return None

        connections = []
        for _, service in self.services.items():
            log().debug('Processing service %s (%s)', service.name, service.id)
            if service.name.startswith('pmm-server'):
                log().info('Ignoring node %s, type %s', service.name, service.type)
                continue
            if target in [service.node.name, service.name]:
                cmd = _cmd(service)
                if extra_args:
                    cmd += ' '.join(extra_args)
                log().debug('Connecting to database: %s', cmd)
                subprocess.run(shlex.split(cmd)) # pylint: disable=subprocess-run-check
            try:
                if not target:
                    name = service.node.name if self.standardise else service.name
                    connections.append(f'{name} address={service.node.address} port={service.port} '
                                       f'distro={service.node.distro} environment={service.node.environment} '
                                       f'service={service.type} cluster={service.cluster}')
            except AttributeError:
                log().error('Unexpected data for service: %s', service)
        return connections

    def ssh(self, target, action='list', extra_args=None) -> list:
        """Connect via SSH"""
        if action == 'connect':
            if not target:
                raise FatalError(f'Unable to use target={target} for SSH')

        connections = []
        for _, node in self.nodes.items():
            log().debug('Processing node %s (%s)', node.name, node.id)
            if node.type != 'generic' or node.name == 'pmm-server':
                log().info('Ignoring node %s, type %s', node.name, node.type)
                continue
            if target == node.name:
                cmd = f'ssh {node.address} -p {node.port} '
                if extra_args:
                    cmd += ' '.join(extra_args)
                log().debug('Connecting to node: %s', cmd)
                subprocess.run(shlex.split(cmd)) # pylint: disable=subprocess-run-check
            try:
                if not target:
                    connections.append(f'{node.name} address={node.address} port={node.port} distro={node.distro}'
                                       f' environment={node.environment}')
            except AttributeError:
                log().error('Unexpected data for node: %s', node)
        return connections


    @staticmethod
    def main(**config):
        """Entrypoint for connection generation"""
        logging.basicConfig(level=config['log_level'].upper())

        if config['sample_config']:
            print(json.dumps(SAMPLE_CONFIG))
            return

        connection_type = {
            'db_connect': 'dbc',
            'ssh_connect': 'ssh',
        }.get(os.path.basename(sys.argv[0]).replace('.py', ''))

        if not connection_type:
            raise FatalError('unknown connection type')

        try:
            runtime_args = shlex.join(sys.argv)
            for k, v in json.load(config['config']).items():
                if k in ['server_address', 'tls_insecure', 'connect_timeout', 'log_level', 'netrc_file',
                         'inventory', 'standardise']:
                    log().debug('Applying override for %s', k)
                    if k.replace('_', '-') not in runtime_args:
                        config[k] = v
            log().setLevel(config['log_level'].upper())
        except (json.JSONDecodeError, OSError):
            pass

        conn = Connect(uri=config['server_address'], verify_tls=not config['tls_insecure'],
                       netrc_path=config['netrc_file'].name, overrides=config.get('inventory'))
        conn.force_requests = config['force']
        conn.standardise = config['standardise']

        if config['list']:
            print('Available connections:')
            for instance in getattr(conn, connection_type)(target=None):
                print(f'{instance}')
        elif len(config['connection_options']):
            for k, v in enumerate(config['connection_options']):
                if k == 0:
                    continue
                config['connection_options'][k] = shlex.quote(v)
                log().debug('Input "%s" sanitised to "%s"', v, config['connection_options'][k])
            syslog.syslog(syslog.LOG_INFO, f"User {conn.username} connecting to "
                                           f"{config['connection_options'][0]} ({connection_type})")
            getattr(conn, connection_type)(target=config['connection_options'][0], action='connect',
                                           extra_args=config['connection_options'][1:])
            syslog.syslog(syslog.LOG_INFO, f"User {conn.username} disconnected from "
                                           f"{config['connection_options'][0]} ({connection_type})")


if __name__ == '__main__':
    try:
        Connect.main(**parse_args())
    except binascii.Error as err:
        log().debug('Reason:', exc_info=True)
        log().error(str(err))
        sys.exit(2)
    except FatalError as err:
        log().debug('Reason:', exc_info=True)
        log().error(str(err))
        sys.exit(1)
