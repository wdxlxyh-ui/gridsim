import logging

logger = logging.getLogger('state')

class MeterModel:
    def __init__(self, config, devices_data, data_lock, start_hour):
        self.config = config
        self.devices_data = devices_data
        self.data_lock = data_lock
        self.start_hour = start_hour
        if not devices_data or not data_lock:
            raise ValueError(f"Invalid initialization for {config.get('DeviceKey')}: devices_data={devices_data is not None}, data_lock={data_lock is not None}")
        self.total_power = 0.0
        logger.info(f"MeterModel initialized for {config.get('DeviceKey')}: Start hour = {self.start_hour}, devices_data length: {len(devices_data)}")

    def update(self):
        try:
            with self.data_lock:
                total_power = 0.0
                pv_power = 0.0
                load_power = 0.0
                for device_key, dev_info in self.devices_data.items():
                    if dev_info.get('model') and 'data' in dev_info and dev_info['data'].get(0) is not None:
                        active_power = dev_info['data'].get(0, 0)
                        if dev_info['type'] == 'PV':
                            pv_power += active_power
                        elif dev_info['type'] in ['Load', 'EV']:
                            load_power += active_power
                        elif dev_info['type'] == 'BESS':
                            if active_power > 0:  # 充电，计入消耗
                                load_power += active_power
                            elif active_power < 0:  # 放电，计入发电
                                pv_power += abs(active_power)
                total_power = load_power - pv_power  # 反转符号，正值表示净消耗，负值表示净发电
                self.total_power = total_power
        except Exception as e:
            logger.error(f"Error in MeterModel update for {self.config.get('DeviceKey')}: {str(e)}")
            self.total_power = 0.0

        logger.debug(f"MeterModel update: PV Power = {pv_power} kW, BESS Discharge = {abs(pv_power - sum(dev['data'].get(0, 0) for dev in self.devices_data.values() if dev['type'] == 'PV')) if pv_power > sum(dev['data'].get(0, 0) for dev in self.devices_data.values() if dev['type'] == 'PV') else 0.0} kW, Load Power = {load_power} kW, Total Power = {self.total_power} kW")
        return {'ActivePower': self.total_power}