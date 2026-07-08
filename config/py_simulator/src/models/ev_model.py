
import logging
import time
logger = logging.getLogger('state')

class EVModel:
    def __init__(self, config, start_hour, devices_data=None, device_key=None):
        self.config = config
        self.rated_power = config.get('PUB_CONN.RatedPW')
        if self.rated_power is None:
            raise ValueError(f"PUB_CONN.RatedPW must be specified for {config.get('DeviceKey', 'EV')} in device.json")
        self.min_charge_pw = config.get('PUB_CONN.MinChargePW', 0.0)
        self.charge_factor = config.get('charge_factor', 1.0)
        self.charge_pw = 0.0
        self.charge_energy_kwh = 0.0
        self.state = 0
        self.start = 0
        self.stop = 0
        self.oem_state = 0
        self.ctrl_state = 0
        self.current_l1 = 0.0
        self.current_l2 = 0.0
        self.current_l3 = 0.0
        self.start_hour = start_hour
        self.charge_schedule = []
        for schedule in config.get('charge_schedule', []):
            start_hour = int(schedule['start'].split(':')[0]) + int(schedule['start'].split(':')[1]) / 60.0
            stop_hour = int(schedule['stop'].split(':')[0]) + int(schedule['stop'].split(':')[1]) / 60.0
            self.charge_schedule.append((start_hour, stop_hour))
        if not self.charge_schedule:
            logger.warning(f"No charge_schedule defined for {config.get('DeviceKey')}, using default 8:00-10:00")
            self.charge_schedule = [(8.0, 10.0)]
        self.start_time = time.time()
        self.charger_type = config.get('charger_type', 'AC').upper()
        self.default_voltage = config.get('voltage', 220.0 if self.charger_type == 'AC' else 400.0)
        self.voltage = self.default_voltage
        self.power_factor = config.get('power_factor', 0.95 if self.charger_type == 'AC' else 1.0)
        self.phase_mode = config.get('phase_mode', 3)  # Default to Three Phase
        if self.charger_type == 'DC':
            self.phase_mode = 1  # DC always single phase
            self.voltage = max(300.0, min(500.0, self.voltage))  # Initial DC voltage limit
        else:
            self.voltage = max(200.0, min(250.0, self.voltage))  # Initial AC voltage limit
        if devices_data and device_key:
            devices_data[device_key]['data'][20] = self.phase_mode
        logger.info(f"EVModel initialized for {config.get('DeviceKey')}: ratedPower = {self.rated_power} kW, minChargePW = {self.min_charge_pw} kW, charge_factor = {self.charge_factor}, Charge schedule = {self.charge_schedule}, Charger type = {self.charger_type}, Voltage = {self.voltage}V, Power Factor = {self.power_factor}, Phase Mode = {'Single' if self.phase_mode == 1 else 'Three'} Phase, Start hour = {self.start_hour}")

    def update(self, devices_data, device_key):
        try:
            elapsed_seconds = time.time() - self.start_time
            current_hour = self.start_hour + (elapsed_seconds / 3600)
            current_hour_display = current_hour % 24
            if current_hour >= 24:
                logger.info(f"EVModel: Time crossed midnight, resetting to {current_hour_display:.4f}h")
            logger.info(f"EVModel update: Elapsed seconds = {elapsed_seconds:.1f}, Current hour = {current_hour_display:.4f}h")
            
            p_limit = devices_data[device_key]['data'].get(8, float('inf'))  # Power setpoint
            charge_cur_set_l1 = devices_data[device_key]['data'].get(10, 0.0)  # Current setpoint
            phase_mode = devices_data[device_key]['data'].get(20, self.phase_mode)  # PhaseMode
            voltage_raw = devices_data[device_key]['data'].get(22, self.voltage * 100)  # Voltage setpoint
            if self.charger_type == 'DC':
                phase_mode = 1  # Force single phase for DC
                self.voltage = max(300.0, min(500.0, voltage_raw / 100.0))  # DC: 300-500V
            else:
                self.voltage = max(200.0, min(250.0, voltage_raw / 100.0))  # AC: 200-250V
            self.phase_mode = phase_mode if phase_mode in [1, 3] else self.config.get('phase_mode', 3)  # Use config default if invalid
            
            self.charge_pw = 0.0
            self.oem_state = 0
            self.ctrl_state = 0
            self.state = 0
            is_charging = False
            for start_hour, stop_hour in self.charge_schedule:
                current_hour_norm = current_hour_display
                start_hour_norm = start_hour
                stop_hour_norm = stop_hour
                if start_hour > stop_hour:
                    if current_hour_norm >= start_hour_norm or current_hour_norm < stop_hour_norm:
                        is_charging = True
                        break
                else:
                    if start_hour_norm <= current_hour_norm < stop_hour_norm:
                        is_charging = True
                        break
            if is_charging:
                base_power = self.rated_power * self.charge_factor
                if charge_cur_set_l1 > 0:  # Current control takes priority
                    if self.charger_type == 'AC' and self.phase_mode == 3:
                        self.charge_pw = charge_cur_set_l1 * 3 * self.voltage * self.power_factor / 1000
                    else:  # AC Single Phase or DC
                        self.charge_pw = charge_cur_set_l1 * self.voltage * self.power_factor / 1000
                else:  # Fallback to power control
                    self.charge_pw = min(base_power, float(p_limit))
                self.charge_pw = max(self.min_charge_pw, min(self.charge_pw, self.rated_power))
                self.oem_state = 1
                self.ctrl_state = 1
                self.state = 1
            if self.charge_pw > 0:
                if self.charger_type == 'AC':
                    if self.phase_mode == 3:  # Three Phase
                        self.current_l1 = self.charge_pw * 1000 / (3 * self.voltage * self.power_factor)
                        self.current_l2 = self.current_l1
                        self.current_l3 = self.current_l1
                        charge_cur_set_l1 = self.current_l1 if charge_cur_set_l1 == 0 else charge_cur_set_l1
                    else:  # Single Phase
                        self.current_l1 = self.charge_pw * 1000 / (self.voltage * self.power_factor)
                        self.current_l2 = 0.0
                        self.current_l3 = 0.0
                        charge_cur_set_l1 = self.current_l1 if charge_cur_set_l1 == 0 else charge_cur_set_l1
                else:  # DC
                    self.current_l1 = self.charge_pw * 1000 / (self.voltage * self.power_factor)
                    self.current_l2 = 0.0
                    self.current_l3 = 0.0
                    charge_cur_set_l1 = self.current_l1 if charge_cur_set_l1 == 0 else charge_cur_set_l1
            else:
                self.current_l1 = 0.0
                self.current_l2 = 0.0
                self.current_l3 = 0.0
                charge_cur_set_l1 = 0.0 if charge_cur_set_l1 == 0 else charge_cur_set_l1
            time_step = 1.0 / 3600.0
            energy_increment = self.charge_pw * time_step
            self.charge_energy_kwh += energy_increment
            if self.charge_pw > 0 and self.start == 0:
                self.start = 1
            if self.charge_pw == 0 and self.stop == 0:
                self.stop = 1
            devices_data[device_key]['data'][0] = self.charge_pw
            devices_data[device_key]['data'][2] = self.current_l1
            devices_data[device_key]['data'][4] = self.current_l2
            devices_data[device_key]['data'][6] = self.current_l3
            devices_data[device_key]['data'][8] = p_limit
            devices_data[device_key]['data'][10] = charge_cur_set_l1
            devices_data[device_key]['data'][12] = self.oem_state
            devices_data[device_key]['data'][14] = self.charge_energy_kwh
            devices_data[device_key]['data'][16] = self.ctrl_state
            devices_data[device_key]['data'][18] = self.state
            devices_data[device_key]['data'][20] = self.phase_mode
            devices_data[device_key]['data'][22] = self.voltage * 100
            logger.info(f"EVModel update: ChargePW = {self.charge_pw} kW, p_limit = {p_limit} kW, Currents = {self.current_l1:.1f}A/{self.current_l2:.1f}A/{self.current_l3:.1f}A, Energy = {self.charge_energy_kwh:.2f} kWh, Start = {self.start}, Stop = {self.stop}, OEM = {self.oem_state}, Ctrl = {self.ctrl_state}, PhaseMode = {'Single' if self.phase_mode == 1 else 'Three'}, ChargeCurSetL1 = {charge_cur_set_l1:.1f}A, Voltage = {self.voltage:.1f}V")
            return {
                'PUB_CONN.ChargePW': self.charge_pw,
                'PUB_CONN.CurrentL1': self.current_l1,
                'PUB_CONN.CurrentL2': self.current_l2,
                'PUB_CONN.CurrentL3': self.current_l3,
                'PUB_CONN.ChargePWSet': p_limit,
                'PUB_CONN.ChargeCurSetL1': charge_cur_set_l1,
                'PUB_CONN.OemState': self.oem_state,
                'PUB_CONN.ChargeEnergyKWH': self.charge_energy_kwh,
                'PUB_CONN.CtrlState': self.ctrl_state,
                'PUB_CONN.State': self.state,
                'PUB_CONN.PhaseMode': self.phase_mode,
                'PUB_CONN.Voltage': self.voltage
            }
        except Exception as e:
            logger.error(f"Error in EVModel update for {self.config.get('DeviceKey')}: {str(e)}")
            self.charge_pw = 0.0
            self.oem_state = 0
            self.ctrl_state = 0
            self.state = 0
            self.current_l1 = 0.0
            self.current_l2 = 0.0
            self.current_l3 = 0.0
            self.phase_mode = self.config.get('phase_mode', 3) if self.charger_type == 'AC' else 1
            self.voltage = self.default_voltage
            return {
                'PUB_CONN.ChargePW': self.charge_pw,
                'PUB_CONN.CurrentL1': self.current_l1,
                'PUB_CONN.CurrentL2': self.current_l2,
                'PUB_CONN.CurrentL3': self.current_l3,
                'PUB_CONN.ChargePWSet': 0.0,
                'PUB_CONN.ChargeCurSetL1': 0.0,
                'PUB_CONN.OemState': self.oem_state,
                'PUB_CONN.ChargeEnergyKWH': self.charge_energy_kwh,
                'PUB_CONN.CtrlState': self.ctrl_state,
                'PUB_CONN.State': self.state,
                'PUB_CONN.PhaseMode': self.phase_mode,
                'PUB_CONN.Voltage': self.voltage
            }