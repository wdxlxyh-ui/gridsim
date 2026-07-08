
# -*- coding: utf-8 -*-
import time
import threading
import logging
import logging.handlers
import json
import os
import sys
from src.communication.modbus_server import ModbusServer
from utils.config_loader import load_devices_config
from utils.locks import data_lock
from src.models.pv_model import PVModel
from src.models.battery_model import BatteryModel
from src.models.ev_model import EVModel
from src.models.meter_model import MeterModel
from src.models.load_model import LoadModel
from config.modbus_registers import REGISTERS

log_config_path = 'config/logging_config.json'
default_config = {
    "log_level": "INFO",
    "log_dir": "log",
    "log_file": "simulation.log",
    "max_size": 10,
    "max_backups": 5,
    "message_log_file": "message.log",
    "message_log_level": "INFO",
    "traffic_log_file": "modbus_traffic.log"
}
log_config = default_config
if os.path.exists(log_config_path):
    try:
        with open(log_config_path, 'r') as f:
            log_config = json.load(f)
    except json.JSONDecodeError as e:
        logging.warning("Invalid JSON in {}: {}. Using default config: {}".format(log_config_path, e, default_config))

log_dir = log_config.get("log_dir", "log")
os.makedirs(log_dir, exist_ok=True)

log_level = getattr(logging, log_config.get("log_level", "INFO").upper(), logging.INFO)
message_log_level = getattr(logging, log_config.get("message_log_level", "INFO").upper(), logging.INFO)

logging.getLogger('').handlers = []
state_logger = logging.getLogger('state')
state_logger.setLevel(log_level)
log_file_path = os.path.join(log_dir, log_config.get("log_file", "simulation.log"))
fh_state = logging.handlers.RotatingFileHandler(
    log_file_path,
    maxBytes=log_config.get("max_size", 10) * 1024 * 1024,
    backupCount=log_config.get("max_backups", 5)
)
fh_state.setLevel(log_level)
formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')
fh_state.setFormatter(formatter)
state_logger.addHandler(fh_state)
state_logger.propagate = False

message_logger = logging.getLogger('message')
message_logger.setLevel(message_log_level)
message_file_path = os.path.join(log_dir, log_config.get("message_log_file", "message.log"))
fh_message = logging.handlers.RotatingFileHandler(
    message_file_path,
    maxBytes=log_config.get("max_size", 10) * 1024 * 1024,
    backupCount=log_config.get("max_backups", 5)
)
fh_message.setLevel(message_log_level)
fh_message.setFormatter(formatter)
message_logger.addHandler(fh_message)
message_logger.propagate = False

traffic_logger = logging.getLogger('traffic')
traffic_logger.setLevel(logging.INFO)
traffic_file_path = os.path.join(log_dir, log_config.get("traffic_log_file", "modbus_traffic.log"))
fh_traffic = logging.handlers.RotatingFileHandler(
    traffic_file_path,
    maxBytes=log_config.get("max_size", 10) * 1024 * 1024,
    backupCount=log_config.get("max_backups", 5)
)
fh_traffic.setLevel(logging.INFO)
fh_traffic.setFormatter(formatter)
traffic_logger.addHandler(fh_traffic)
traffic_logger.propagate = False

logger = logging.getLogger(__name__)
devices_data = {}
START_HOUR = 0.0
modbus_server = None
simulation_start_time = None

def load_config():
    global devices_data, START_HOUR, simulation_start_time
    config = load_devices_config('config/device.json')
    start_time = config['start_time']
    hh, mm = map(int, start_time.split(':'))
    START_HOUR = hh + mm / 60.0
    simulation_start_time = time.time()
    state_logger.info("Simulation start hour set to {}, start time: {}".format(START_HOUR, time.ctime(simulation_start_time)))
    slave_ids = set()
    next_slave_id = 1
    for device_type in config.get('Devices', []):
        for dev_type, dev_list in device_type.items():
            for dev in dev_list:
                device_key = dev['DeviceKey']
                initial_data = {}
                for reg_offset in range(0, 100, 2):
                    initial_data[reg_offset] = 0.0
                for reg_name, reg_offset in REGISTERS.get(dev_type, {}).get('points', {}).items():
                    initial_data[reg_offset] = 0.0
                    if dev_type == 'EV' and reg_name == 'PUB_CONN.ChargePWSet':
                        initial_data[reg_offset] = dev.get('PUB_CONN.RatedPW', 22.0)
                configured_slave_id = dev.get('slave_id')
                if configured_slave_id is not None:
                    if not 1 <= configured_slave_id <= 247:
                        state_logger.error("Invalid slave_id {} for {}, must be 1-247. Using auto-assigned ID.".format(configured_slave_id, device_key))
                        while next_slave_id in slave_ids:
                            next_slave_id += 1
                            if next_slave_id > 247:
                                state_logger.error("No available slave_id (1-247)")
                                return
                        slave_id = next_slave_id
                        next_slave_id += 1
                    elif configured_slave_id in slave_ids:
                        state_logger.error("Duplicate slave_id {} for {}. Using auto-assigned ID.".format(configured_slave_id, device_key))
                        while next_slave_id in slave_ids:
                            next_slave_id += 1
                            if next_slave_id > 247:
                                state_logger.error("No available slave_id (1-247)")
                                return
                        slave_id = next_slave_id
                        next_slave_id += 1
                    else:
                        slave_id = configured_slave_id
                else:
                    while next_slave_id in slave_ids:
                        next_slave_id += 1
                        if next_slave_id > 247:
                            state_logger.error("No available slave_id (1-247)")
                            return
                    slave_id = next_slave_id
                    next_slave_id += 1
                slave_ids.add(slave_id)
                devices_data[device_key] = {
                    'type': dev_type,
                    'data': initial_data,
                    'config': dev,
                    'model': None,
                    'slave_id': slave_id,
                    'is_idle': False
                }
                try:
                    if dev_type == 'PV':
                        devices_data[device_key]['model'] = PVModel(dev, START_HOUR)
                    elif dev_type == 'BESS':
                        devices_data[device_key]['model'] = BatteryModel(dev, START_HOUR, devices_data, device_key, data_lock)
                    elif dev_type == 'EV':
                        devices_data[device_key]['model'] = EVModel(dev, START_HOUR, devices_data, device_key)
                    elif dev_type == 'Load':
                        devices_data[device_key]['model'] = LoadModel(dev, START_HOUR)
                    elif dev_type == 'Meter':
                        state_logger.debug("Initializing MeterModel for {}".format(device_key))
                        devices_data[device_key]['model'] = MeterModel(dev, devices_data, data_lock, START_HOUR)
                        state_logger.debug("MeterModel initialized for {}".format(device_key))
                except Exception as e:
                    state_logger.error("Failed to initialize model for {}: {} with config {}".format(device_key, str(e), dev))
    state_logger.info("Loaded devices with slave IDs: {}".format([(k, v['slave_id']) for k, v in devices_data.items()]))
    return devices_data

def update_device_data():
    global modbus_server, simulation_start_time
    prev_pv_total = 0.0
    last_linkage_time = time.time()
    last_log_timestamp = time.time() - 1.0
    log_interval = 1.0
    while True:
        current_time = time.time()
        elapsed_seconds = current_time - simulation_start_time
        current_hour = START_HOUR + (elapsed_seconds / 3600)
        current_hour_display = current_hour % 24
        try:
            state_logger.debug("Starting update cycle, current_hour: {:.2f} (display: {:.2f}), devices_data length: {}".format(current_hour, current_hour_display, len(devices_data)))
            for device_key, dev_info in devices_data.items():
                if dev_info['model'] and dev_info['type'] != 'Meter':
                    try:
                        if dev_info['type'] == 'PV':
                            new_data = dev_info['model'].update(p_limit=dev_info['data'].get(4, 0))
                        elif dev_info['type'] == 'BESS':
                            new_data = dev_info['model'].update(p_command=dev_info['data'].get(14, 0))
                        elif dev_info['type'] == 'EV':
                            new_data = dev_info['model'].update(devices_data, device_key)
                        elif dev_info['type'] == 'Load':
                            new_data = dev_info['model'].update()
                        points = REGISTERS.get(dev_info['type'], {}).get('points', {})
                        for reg_name, reg_offset in points.items():
                            if reg_name in new_data:
                                dev_info['data'][reg_offset] = new_data[reg_name]
                                state_logger.debug("Updated {} {} at {} to {}".format(device_key, reg_name, reg_offset, new_data[reg_name]))
                    except Exception as e:
                        state_logger.error("Error updating {}: {} with traceback {}".format(device_key, str(e), e.__traceback__))
            for device_key, dev_info in devices_data.items():
                if dev_info['type'] == 'Meter' and dev_info['model']:
                    try:
                        state_logger.debug("Updating Meter {}, devices_data length: {}".format(device_key, len(devices_data)))
                        new_data = dev_info['model'].update()
                        for reg_name, value in REGISTERS.get(dev_info['type'], {}).get('points', {}).items():
                            if reg_name in new_data:
                                dev_info['data'][value] = new_data[reg_name]
                                state_logger.debug("Updated {} {} at {} to {} kW".format(device_key, reg_name, value, new_data[reg_name]))
                    except Exception as e:
                        state_logger.error("Error updating {}: {} with traceback {}".format(device_key, str(e), e.__traceback__))
                elif not dev_info['model']:
                    state_logger.error("Model for {} is None, skipping update".format(device_key))
            total_power = 0.0
            for dev_info in devices_data.values():
                if dev_info['type'] == 'Meter':
                    total_power += dev_info['data'].get(0, 0)
        except Exception as e:
            state_logger.error("Error in update_device_data: {} with traceback {}".format(str(e), e.__traceback__))
            continue
        if current_time - last_linkage_time >= 1.0:
            with data_lock:
                pv_total = sum(dev['data'].get(0, 0) for dev in devices_data.values() if dev['type'] == 'PV')
                prev_pv_total = pv_total
            last_linkage_time = current_time
        if current_time - last_log_timestamp >= log_interval:
            with data_lock:
                log_messages = []
                for device_key, dev_info in devices_data.items():
                    if dev_info['type'] == 'Meter':
                        log_messages.append("Meter {} Total Power: {:.2f} kW".format(device_key, dev_info['data'].get(0, 0)))
                    elif dev_info['type'] == 'PV':
                        log_messages.append("PV {} GenActivePW: {:.2f} kW, LimitPower: {:.2f} kW".format(device_key, dev_info['data'].get(0, 0), dev_info['data'].get(4, 0)))
                    elif dev_info['type'] == 'BESS':
                        log_messages.append("BESS {} ActivePW: {:.2f} kW, SOC: {:.1f}%, ChargingEng: {:.2f} kWh, DischargingEng: {:.2f} kWh".format(device_key, dev_info['data'].get(0, 0), dev_info['data'].get(2, 0), dev_info['data'].get(20, 0), dev_info['data'].get(22, 0)))
                    elif dev_info['type'] == 'EV':
                        log_messages.append("EV {} ChargePW: {:.2f} kW".format(device_key, dev_info['data'].get(0, 0)))
                    elif dev_info['type'] == 'Load':
                        log_messages.append("Load {} Power: {:.2f} kW".format(device_key, dev_info['data'].get(0, 0)))
                if log_messages:
                    state_logger.info("Device States: {}".format(" | ".join(log_messages)))
                    state_logger.debug("Log messages generated: {}".format(log_messages))
            last_log_timestamp = current_time
        time.sleep(1.0)

def get_cli_port():
    """Parse --port argument from command line, default 5021."""
    for i, arg in enumerate(sys.argv):
        if arg == '--port' and i + 1 < len(sys.argv):
            try:
                return int(sys.argv[i + 1])
            except ValueError:
                pass
    return int(os.environ.get('MODBUS_PORT', '5021'))


def main():
    global modbus_server
    state_logger.info("Starting MicroGrid Simulator...")
    load_config()
    state_logger.info("Loaded devices: {}".format(list(devices_data.keys())))
    port = get_cli_port()
    modbus_server = ModbusServer(devices_data, data_lock, port=port)
    state_logger.info("Initializing Modbus server...")
    server_thread = threading.Thread(target=modbus_server.run, daemon=False)
    server_thread.start()
    state_logger.info("Modbus server thread started")
    state_logger.info("Starting update thread...")
    update_thread = threading.Thread(target=update_device_data, daemon=False)
    update_thread.start()
    state_logger.info("Update thread started")
    try:
        while True:
            if not server_thread.is_alive():
                state_logger.error("Modbus server thread terminated unexpectedly")
                break
            if not update_thread.is_alive():
                state_logger.error("Update thread terminated unexpectedly")
                break
            time.sleep(1)
    except KeyboardInterrupt:
        state_logger.info("Shutting down MicroGrid Simulator...")
    finally:
        state_logger.info("Shutting down MicroGrid Simulator...")
        if modbus_server and modbus_server.sock:
            modbus_server.sock.close()
        update_thread.join(timeout=5)
        server_thread.join(timeout=5)

if __name__ == "__main__":
    main()