import logging
logger = logging.getLogger('state')

class BatteryModel:
    def __init__(self, config, start_hour, devices_data=None, device_key=None, data_lock=None):
        self.config = config
        self.devices_data = devices_data
        self.device_key = device_key
        self.data_lock = data_lock
        self.rated_capacity = config.get('ratedCapacity', 200)
        self.max_charge_power = config.get('maxChargePower', 100)
        self.max_discharge_power = config.get('maxDischargePower', 100)
        self.soc_max = config.get('socMax', 0.9)
        self.soc_min = config.get('socMin', 0.1)
        self.initial_soc = config.get('initial_soc', 0.5)
        self.voltage_nominal = config.get('voltage_nominal', 48)
        self.resistance = config.get('resistance', 0.01)
        self.soc = min(self.soc_max * 100.0, max(self.soc_min * 100.0, self.initial_soc * 100.0))
        self.active_power = 0.0
        self.total_charging_eng = 0.0
        self.total_discharging_eng = 0.0
        self.start_hour = start_hour
        
        if self.devices_data and self.device_key and self.data_lock:
            with self.data_lock:
                self.devices_data[self.device_key]['data'][2] = self.soc
                
        logger.info(f"BatteryModel initialized for {config.get('DeviceKey')}: ratedCapacity = {self.rated_capacity} kWh, Start hour = {self.start_hour}, Initial SOC = {self.soc:.1f}%, SOC range = [{self.soc_min * 100.0:.1f}, {self.soc_max * 100.0:.1f}]%")

    def update(self, p_command=0.0):
        try:
            if self.devices_data and self.device_key and self.data_lock:
                with self.data_lock:
                    soc_raw = self.devices_data[self.device_key]['data'].get(2, None)
                    if soc_raw is not None:
                        new_soc = max(self.soc_min * 100.0, min(self.soc_max * 100.0, soc_raw))
                        if abs(new_soc - self.soc) > 0.1:
                            self.soc = new_soc
                            logger.info(f"BatteryModel: SOC directly set to {self.soc:.1f}%")

            p_command = max(-self.max_discharge_power, min(self.max_charge_power, p_command))
            self.active_power = p_command
            
            time_step = 1.0 / 3600.0
            energy_increment = self.active_power * time_step
            soc_increment = (energy_increment / self.rated_capacity) * 100.0
            new_soc = self.soc + soc_increment

            if self.active_power > 0:
                if new_soc > self.soc_max * 100.0:
                    excess_soc = new_soc - self.soc_max * 100.0
                    excess_energy = (excess_soc / 100.0) * self.rated_capacity
                    energy_increment -= excess_energy
                    soc_increment = (energy_increment / self.rated_capacity) * 100.0
                    new_soc = self.soc_max * 100.0
                self.total_charging_eng += energy_increment
                self.soc = min(self.soc_max * 100.0, self.soc + soc_increment)
            elif self.active_power < 0:
                if new_soc < self.soc_min * 100.0:
                    excess_soc = self.soc_min * 100.0 - new_soc
                    excess_energy = (excess_soc / 100.0) * self.rated_capacity
                    energy_increment += excess_energy
                    soc_increment = (energy_increment / self.rated_capacity) * 100.0
                    new_soc = self.soc_min * 100.0
                self.total_discharging_eng -= energy_increment
                self.soc = max(self.soc_min * 100.0, self.soc + soc_increment)
            else:
                soc_increment = 0.0

            self.soc = min(self.soc_max * 100.0, max(self.soc_min * 100.0, self.soc))

            if self.devices_data and self.device_key and self.data_lock:
                with self.data_lock:
                    self.devices_data[self.device_key]['data'][2] = self.soc

            logger.debug(f"BatteryModel update: ActivePW = {self.active_power} kW, Energy increment = {energy_increment:.6f} kWh, TotalChargingEng = {self.total_charging_eng:.2f} kWh, TotalDischargingEng = {self.total_discharging_eng:.2f} kWh, SOC increment = {soc_increment:.6f}%, new SOC = {self.soc:.1f}%")

            return {
                'BS.ActivePW': self.active_power,
                'BS.Soc': self.soc,
                'BS.MaxChargePower': self.max_charge_power,
                'BS.MaxDischargePower': self.max_discharge_power,
                'BS.EndChargeSOC': self.soc_max * 100.0,
                'BS.EndDischargeSOC': self.soc_min * 100.0,
                'BS.Soh': 100.0,
                'BS.SysAPSetPoint': p_command,
                'BS.OemState': 1,
                'BS.CtrlState': 1,
                'BS.TotalChargingEng': self.total_charging_eng,
                'BS.TotalDischargingEng': self.total_discharging_eng
            }
        except Exception as e:
            logger.error(f"Error in BatteryModel update for {self.config.get('DeviceKey')}: {str(e)}")
            self.active_power = 0.0
            return {
                'BS.ActivePW': self.active_power,
                'BS.Soc': self.soc,
                'BS.MaxChargePower': self.max_charge_power,
                'BS.MaxDischargePower': self.max_discharge_power,
                'BS.EndChargeSOC': self.soc_max * 100.0,
                'BS.EndDischargeSOC': self.soc_min * 100.0,
                'BS.Soh': 100.0,
                'BS.SysAPSetPoint': p_command,
                'BS.OemState': 1,
                'BS.CtrlState': 1,
                'BS.TotalChargingEng': self.total_charging_eng,
                'BS.TotalDischargingEng': self.total_discharging_eng
            }