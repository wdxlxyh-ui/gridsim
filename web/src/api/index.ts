import axios from 'axios'

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

const TOKEN_KEY = 'iec104_token'
export function getToken(): string | null { return localStorage.getItem(TOKEN_KEY) }
export function setToken(token: string) { localStorage.setItem(TOKEN_KEY, token) }
export function clearToken() { localStorage.removeItem(TOKEN_KEY) }
export async function login(username: string, password: string): Promise<{ token: string }> {
  const res = await axios.post('/api/v1/auth/login', { username, password })
  return res.data
}

export interface ModbusConfig {
  port?: number
  byte_order?: string
  slave_id?: number
}

export interface InstanceConfig {
  id?: string
  name: string
  iec104_port: number
  xlsx_file: string
  enabled?: boolean
  http_enabled?: boolean
  http_port?: number
  protocol?: string
  modbus_config?: ModbusConfig
}

export interface InstanceStats {
  uptime_seconds: number
  total_points: number
  client_connected: boolean
  interrogations: number
  controls: number
  spontaneous: number
}

export interface InstanceState {
  id: string
  name: string
  iec104_port: number
  xlsx_file: string
  enabled: boolean
  http_enabled?: boolean
  http_port?: number
  protocol?: string
  status: 'running' | 'stopped' | 'error'
  stats?: InstanceStats
  error?: string
}

export interface GlobalStatus {
  version: string
  mode: string
  configured: number
  running: number
  stopped: number
  max: number
}

// Instance CRUD
export async function listInstances(): Promise<InstanceState[]> {
  const res = await http.get('/instances')
  return res.data.instances
}

export async function createInstance(cfg: InstanceConfig): Promise<void> {
  await http.post('/instances', cfg)
}

export async function getInstance(id: string): Promise<InstanceState> {
  const res = await http.get(`/instances/${id}`)
  return res.data
}

export async function updateInstance(id: string, cfg: InstanceConfig): Promise<void> {
  await http.put(`/instances/${id}`, cfg)
}

export async function deleteInstance(id: string): Promise<void> {
  await http.delete(`/instances/${id}`)
}

// Instance control
export async function startInstance(id: string): Promise<void> {
  await http.post(`/instances/${id}/start`)
}

export async function stopInstance(id: string): Promise<void> {
  await http.post(`/instances/${id}/stop`)
}

export async function restartInstance(id: string): Promise<void> {
  await http.post(`/instances/${id}/restart`)
}

// File upload
export async function uploadExcel(file: File): Promise<string> {
  const form = new FormData()
  form.append('file', file)
  const res = await http.post('/upload', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data.filename
}

// Global status
export async function getStatus(): Promise<GlobalStatus> {
  const res = await http.get('/status')
  return res.data
}

// Uploaded files list
export async function listFiles(): Promise<{ name: string; size: number; modtime: string }[]> {
  const res = await http.get('/files')
  return res.data.files
}

export interface PointSnapshot {
  ioa: number
  name: string
  point_type: string
  value: number
  bool_value: boolean
  int_value: number
  updated_at: string
  unit: string
  function_code?: number
  register_address?: number
  byte_order?: string
}

export interface PointsResponse {
  points: PointSnapshot[]
  refreshed_at: string
}

export interface StrategyParams {
   start_value?: number
   step?: number
   period_ms?: number
   max_value?: number
   min_value?: number
   max_value_r?: number
   decimal_places?: number
   csv_file?: string
   time_format?: string
   time_unit?: string
   csv_column_map?: string
   csv_loop?: boolean
   para_a?: string
   para_b?: string
   init_soc?: number
   rated_cap?: number
   power_ioa?: number
   integral_ms?: number
   init_energy?: number
   stat_type?: number
   energy_power_ioa?: number
   energy_period_ms?: number
   follow_ao_ioa?: number
   api_init_value?: number
   custom_ioas?: string
   custom_formula?: string
  }

export interface AutoChangeConfig {
  ioa: number
  strategy: string
  enabled: boolean
  params: StrategyParams
  updated_at: string
}

export interface BatchAutoChangeRequest {
  ioas: number[]
  config: {
    strategy: string
    enabled: boolean
    params: StrategyParams
  }
}

export async function getPoints(instanceId: string): Promise<PointsResponse> {
  const res = await http.get(`/instances/${instanceId}/points`)
  return res.data
}

export async function readPoint(instanceId: string, ioa: number): Promise<PointSnapshot> {
  const res = await http.get(`/instances/${instanceId}/points/${ioa}`)
  return res.data
}

export async function readPointsBatch(instanceId: string, ioas: number[]): Promise<PointsResponse> {
  const res = await http.get(`/instances/${instanceId}/points/batch`, {
    params: { ioas: ioas.join(',') },
  })
  return res.data
}

export async function setPointValue(instanceId: string, ioa: number, value: any): Promise<any> {
  const res = await http.put(`/instances/${instanceId}/points/${ioa}`, value)
  return res.data
}

export async function getAutoChange(instanceId: string, ioa: number): Promise<AutoChangeConfig> {
  const res = await http.get(`/instances/${instanceId}/points/auto-change/${ioa}`)
  return res.data
}

export async function setAutoChange(instanceId: string, ioa: number, cfg: any): Promise<any> {
  const res = await http.put(`/instances/${instanceId}/points/auto-change/${ioa}`, cfg)
  return res.data
}

export async function deleteAutoChange(instanceId: string, ioa: number): Promise<any> {
  const res = await http.delete(`/instances/${instanceId}/points/auto-change/${ioa}`)
  return res.data
}

export async function batchAutoChange(instanceId: string, req: BatchAutoChangeRequest): Promise<any> {
  const res = await http.put(`/instances/${instanceId}/points/auto-change/batch`, req)
  return res.data
}

export async function exportAutoConfig(instanceId: string): Promise<Blob> {
  const res = await http.get(`/instances/${instanceId}/points/auto-change/export`, { responseType: 'blob' })
  return res.data
}

export async function importAutoConfig(instanceId: string, file: File): Promise<any> {
  const form = new FormData()
  form.append('file', file)
  const res = await http.post(`/instances/${instanceId}/points/auto-change/import`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data
}

export async function exportPointsCSV(instanceId: string): Promise<Blob> {
  const res = await http.get(`/instances/${instanceId}/points/export`, { responseType: 'blob' })
  return res.data
}

export async function uploadCSV(instanceId: string, file: File): Promise<any> {
  const form = new FormData()
  form.append('file', file)
  const res = await http.post(`/instances/${instanceId}/upload-csv`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data
}

export async function getProtocols(): Promise<string[]> {
  const res = await http.get('/protocols')
  return res.data.protocols
}

export interface CSVFileInfo {
  name: string
  size: number
  modtime: string
  shared: boolean
}

export async function listCSVFiles(instanceId: string): Promise<CSVFileInfo[]> {
  const res = await http.get(`/instances/${instanceId}/csv-files`)
  return res.data.files
}

export async function readCSVHeaders(instanceId: string, filename: string): Promise<string> {
  const res = await http.get(`/instances/${instanceId}/csv-content/${encodeURIComponent(filename)}`)
  return res.data.content
}

export interface CSVReplayMapping {
  column: number
  ioa: number
}

export async function configCSVReplay(instanceId: string, csvFile: string, mappings: CSVReplayMapping[], timeFormat?: string, timeUnit?: string): Promise<any> {
  const res = await http.post(`/instances/${instanceId}/csv-replay`, {
    csv_file: csvFile,
    time_format: timeFormat || 'relative',
    time_unit: timeUnit || 'ms',
    mappings,
  })
  return res.data
}

// ─── Microgrid API ────────────────────────────────────────────────────────

export interface MicrogridDeviceSwitch {
  id: string
  name: string
  closed: boolean
  controllable: boolean
}

export interface MicrogridDeviceParams {
  rated_power_kw?: number
  efficiency?: number
  capacity_kwh?: number
  rated_power_kw_b?: number
  init_soc?: number
  soc_min?: number
  soc_max?: number
  eff?: number
  load_rated_kw?: number
  power_factor?: number
  charger_rated_kw?: number
  charger_eff?: number
}

export interface MicrogridCustomPoint {
  name: string
  type: 'AI' | 'DI' | 'DO' | 'AO'
  alias?: string
}

export interface MicrogridStrategyConfig {
  type: string
  enabled: boolean
  params?: Record<string, any>
}

export interface MicrogridDevice {
  id: string
  type: 'pv' | 'battery' | 'load' | 'charger'
  name: string
  switch: MicrogridDeviceSwitch
  params: MicrogridDeviceParams
  ioa_base?: number
  power?: number
  soc?: number
  control_mode?: 'remote' | 'local'
  strategy?: MicrogridStrategyConfig
  custom_points?: MicrogridCustomPoint[]
}

export interface GridMeterConfig {
  rated_capacity_kw: number
  island_mode: boolean
}

export interface MicrogridTopology {
  grid_meter: GridMeterConfig
  bus_name: string
  bus_voltage_kv: number
  devices: MicrogridDevice[]
}

export interface MicrogridDashboard {
  status?: string
  grid_power_kw: number
  total_pv_kw: number
  total_bat_kw: number
  total_load_kw: number
  total_charger_kw: number
  battery_soc?: number
  pv?: { id: string; name: string; power_kw: number; closed: boolean; mode?: string }[]
  battery?: { id: string; name: string; power_kw: number; closed: boolean; soc?: number; mode?: string }[]
  load?: { id: string; name: string; power_kw: number; closed: boolean; mode?: string }[]
  charger?: { id: string; name: string; power_kw: number; closed: boolean; mode?: string }[]
  device_count?: number
}

export async function getMicrogridTopology(instanceId: string): Promise<MicrogridTopology> {
  const res = await http.get(`/microgrid/${instanceId}/topology`)
  return res.data
}

export async function saveMicrogridTopology(instanceId: string, topo: MicrogridTopology): Promise<void> {
  await http.put(`/microgrid/${instanceId}/topology`, topo)
}

export async function addMicrogridDevice(instanceId: string, device: Partial<MicrogridDevice>): Promise<MicrogridDevice> {
  const res = await http.post(`/microgrid/${instanceId}/device`, device)
  return res.data
}

export async function updateMicrogridDevice(instanceId: string, device: MicrogridDevice): Promise<void> {
  await http.put(`/microgrid/${instanceId}/device`, device)
}

export async function deleteMicrogridDevice(instanceId: string, devId: string): Promise<void> {
  await http.delete(`/microgrid/${instanceId}/device/${devId}`)
}

export async function controlMicrogridSwitch(instanceId: string, devId: string, closed: boolean): Promise<void> {
  await http.post(`/microgrid/${instanceId}/control/${devId}?closed=${closed}`)
}

export async function getMicrogridDashboard(instanceId: string): Promise<MicrogridDashboard> {
  const res = await http.get(`/microgrid/${instanceId}/dashboard`)
  return res.data
}

export async function getMicrogridPoints(instanceId: string): Promise<{ points: any[] }> {
  const res = await http.get(`/microgrid/${instanceId}/points`)
  return res.data
}

// ── Microgrid Formulas ──

export interface MicrogridFormula {
  id: string
  name: string
  target: string
  expression: string
  enabled: boolean
}

export async function getMicrogridFormulas(instanceId: string): Promise<MicrogridFormula[]> {
  const res = await http.get(`/microgrid/${instanceId}/formulas`)
  return res.data
}

export async function addMicrogridFormula(instanceId: string, formula: Partial<MicrogridFormula>): Promise<MicrogridFormula> {
  const res = await http.post(`/microgrid/${instanceId}/formulas`, formula)
  return res.data
}

export async function updateMicrogridFormula(instanceId: string, formula: MicrogridFormula): Promise<void> {
  await http.put(`/microgrid/${instanceId}/formulas`, formula)
}

export async function deleteMicrogridFormula(instanceId: string, formulaId: string): Promise<void> {
  await http.delete(`/microgrid/${instanceId}/formulas/${formulaId}`)
}

// ─── Proxy API Tester ──────────────────────────────────────────────────────

export interface ProxyRequest {
  method: string
  url: string
  headers: Record<string, string>
  body: string
  timeout?: number
}

export interface ProxyResponse {
  status: number
  status_text: string
  headers: Record<string, string>
  body: string
  time_ms: number
  size: number
  error?: string
}

export interface CollectionItem {
  id: string
  name: string
  type: 'folder' | 'request'
  method?: string
  url?: string
  headers?: Record<string, string>
  body?: string
  pre_script?: string
  children?: CollectionItem[]
}

export interface ProxyEnvironment {
  id: string
  name: string
  variables: Record<string, string>
}

export async function proxyRequest(req: ProxyRequest): Promise<ProxyResponse> {
  const res = await http.post('/proxy', req, { timeout: 120000 })
  return res.data
}

export async function getCollections(): Promise<CollectionItem[]> {
  const res = await http.get('/proxy/collections')
  return res.data.collections
}

export async function saveCollection(item: CollectionItem): Promise<CollectionItem> {
  const res = await http.post('/proxy/collections', item)
  return res.data
}

export async function deleteCollection(id: string): Promise<void> {
  await http.delete(`/proxy/collections/${id}`)
}

export async function getEnvironments(): Promise<{ environments: ProxyEnvironment[]; active_id: string }> {
  const res = await http.get('/proxy/environments')
  return res.data
}

export async function saveEnvironment(env: ProxyEnvironment): Promise<ProxyEnvironment> {
  const res = await http.post('/proxy/environments', env)
  return res.data
}

export async function deleteEnvironment(id: string): Promise<void> {
  await http.delete(`/proxy/environments/${id}`)
}

export async function activateEnvironment(id: string): Promise<void> {
  await http.post(`/proxy/environments/${id}/activate`)
}
