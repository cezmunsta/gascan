#!/usr/bin/env python3
"""
Dynamic inventory for Ansible

This is a sample script that can be used as-is with a config,
or as a base to create one that meets the requirements of the
environment where gascan will be used.
"""

from argparse import (
    ArgumentDefaultsHelpFormatter,
    ArgumentParser,
    FileType,
)
from http import HTTPStatus
import json
import logging
import os
from stat import (
    S_IEXEC,
    S_IREAD,
)
import shlex
from subprocess import (
    CalledProcessError,
    run,
)
import sys
import time
import urllib.request


DEFAULT_CONFIG_FILE = 'config.json'
DEFAULT_HEADERS = {
    'Content-type': 'application/json',
}
DEFAULT_KEY_FILE = None
DEFAULT_LOG_LEVEL = logging.WARNING
DEFAULT_MAX_ATTEMPTS = 3
DEFAULT_REQUEST_TIMEOUT = 10
DEFAULT_RETRY_WAIT_SECONDS = 10
DEFAULT_URI = 'http://localhost/inventory'

SAMPLE_CONFIG = json.dumps({
    'headers': DEFAULT_HEADERS,
    'key_file': DEFAULT_KEY_FILE,
    'retry_attempts': DEFAULT_MAX_ATTEMPTS,
    'retry_wait_seconds': DEFAULT_RETRY_WAIT_SECONDS,
    'uri': DEFAULT_URI,
}, indent=4)
SAMPLE_READ_INVENTORY = b"""#!/bin/bash

set -eu

declare DATA="${1:-inventory.json}"
declare KEY_FILE="${2:-.vault-key}"

trap "rm -f "${DATA}"" EXIT

PEX_SCRIPT=ansible-vault \
ANSIBLE_VAULT_PASSWORD_FILE="${KEY_FILE}" \
    ansible decrypt "${DATA}" &>/dev/null
cat "${DATA}"

"""

logging.basicConfig(level=DEFAULT_LOG_LEVEL,
                    format='%(asctime)s %(levelname)s:%(name)s: PID<%(process)d> '
                           '%(module)s.%(funcName)s - %(message)s')
LOG = logging.getLogger('get_inventory')


class InventoryConfig:  # pylint: disable=too-few-public-methods
    """
    Configuration for inventory requests
    """
    log_level = DEFAULT_LOG_LEVEL
    key_file = DEFAULT_KEY_FILE
    headers = DEFAULT_HEADERS
    request_timeout = DEFAULT_REQUEST_TIMEOUT
    retry_attempts = DEFAULT_MAX_ATTEMPTS
    retry_wait_seconds = DEFAULT_RETRY_WAIT_SECONDS
    uri = DEFAULT_URI

    def __init__(self, **kwargs) -> None:
        """
        Initialiase the InventoryConfig

        :param **kwargs: any of the defined properties for the class

        :raises: ValueError: if key_file is unset, or not a file
        """
        for k, v in kwargs.items():
            if hasattr(self, k):
                setattr(self, k, v)
        self.set_logging()

        if not os.path.isfile(self.key_file):
            logging.warning('unable to find, or read the key: "%s"', self.key_file)
            raise ValueError('key_file should set to the path for the key')

    def get_key(self) -> bytes:
        """
        Read the key from the key file

        >>> cfg = InventoryConfig(key_file='/path/to/file.json')
        >>> print(cfg.get_key().decode())

        :return: the key
        :rtype: bytes
        """
        with open(self.key_file, 'rb') as key_file:
            return key_file.read().decode()

    def set_logging(self) -> None:
        """
        Modify the logger
        """
        if LOG.level == self.log_level:
            return
        LOG.setLevel(self.log_level)


def configure(config_file:FileType, generate_sample:bool) -> InventoryConfig:
    """
    Generate a configuration instance from a JSON configuration file.

    The minimum content for the configuration file is to define key_file,
    else the InventoryConfig initialisation will raise an exception.

    >>> cfg = configure(config_file="/path/to/file.json")
    >>> print(cfg.uri)

    :param config_file: the path to the config file
    :type path: str
    :param generate_sample: generate a sample config to get started
    :type generate_sample: bool

    :raises: json.decoder.JSONDecodeError: if the config is not valid JSON
    :raises: OSError: if the config cannot be accessed

    :return: the configuration instance
    :rtype: InventoryConfig
    """
    if generate_sample:
        print(SAMPLE_CONFIG)
        sys.exit(0)
    try:
        cfg = InventoryConfig(**json.load(config_file))
    except json.decoder.JSONDecodeError:
        logging.error('the config file "%s" does not appear to be valid JSON', config_file)
        raise
    except OSError:
        logging.error('unable to access the config file "%s"', config_file)
        raise
    return cfg


def parse_args() -> dict:
    """
    Parse command arguments

    :return: the configuration from the command arguments
    :rtype: dict
    """
    parser = ArgumentParser(formatter_class=ArgumentDefaultsHelpFormatter)
    parser.add_argument('--config-file', '-c', type=FileType('rb'),
                        default=os.getenv('GASCAN_INVENTORY_CONFIG_FILE', DEFAULT_CONFIG_FILE),
                        help='Path to the JSON config file')
    parser.add_argument('--generate-sample', '-s', action='store_true',
                        help='Print out a sample config')
    return vars(parser.parse_args())


def main():
    """
    Call the dummy inventory and return the first customers

    TODO: - add caching of the encrypted inventory
            - use for CACHE_LIFETIME
            - if exceeding CACHE_LIFETIME and unable to make a request, use cache
          - add support for direct Ansible decryption via API

    :return: customer inventory
    """
    cfg = configure(**parse_args())
    key_file = cfg.key_file

    LOG.debug('building inventory request')
    req = urllib.request.Request(url=cfg.uri, method='POST',
                                 data=json.dumps(dict(key=cfg.get_key())).encode('utf-8'))
    for header, value in cfg.headers.items():
        req.add_header(header, value)

    attempt = 0
    resp = None
    LOG.debug('requesting inventory')
    while attempt < cfg.retry_attempts:
        try:
            with urllib.request.urlopen(req, timeout=cfg.request_timeout) as resp:
                status = resp.status
                if status == HTTPStatus.OK:
                    data = resp.read()
                    break
                LOG.error('failed to request inventory: %d', resp.status)
        except: # pylint: disable=bare-except
            LOG.warning('failed attempt = %d', attempt, exc_info=True)
        time.sleep(cfg.retry_wait_seconds)
        attempt += 1
        LOG.debug('attempts = %d', attempt)
        data = b'{}'
    del cfg
    del req
    del resp

    LOG.debug('checking for read_inventory.sh')
    if not os.path.isfile('read_inventory.sh'):
        LOG.debug('generating read_inventory.sh')
        with open('read_inventory.sh', 'wb') as read_inventory:
            read_inventory.write(SAMPLE_READ_INVENTORY)
            os.fchmod(read_inventory.fileno(), S_IREAD | S_IEXEC)

    LOG.debug('generating inventory.json')
    with open('inventory.json', 'wb') as inventory:
        inventory.write(data)

    LOG.debug('process inventory.json')
    try:
        cmd = run(shlex.split(f'bash read_inventory.sh inventory.json {key_file}'),
                  check=True, capture_output=True)
        return cmd.stdout.decode()
    except CalledProcessError:
        LOG.error('failed to read the inventory from disk', exc_info=True)
        return json.dumps(dict(error=1))


if __name__ == '__main__':
    # noinspection PyBroadException
    try:
        print(main())
    except SystemExit:
        pass
    except:  # pylint: disable=bare-except
        LOG.exception('failed to successfully complete the request')
