import Alpine from 'alpinejs'

function getBaseURL() {
  const currentURL = new URL(window.location.href)
  if (currentURL.port === '4321') {
    return 'http://localhost:3003'
  }
  return `${currentURL.origin}${currentURL.pathname}`.replace(/\/$/, '')
}

function formatBytes(bytes, decimals = 2) {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`
}

function numberToK(number) {
  return number > 999 ? `${(number / 1000).toFixed(1)}k` : number
}

function formatLogString(value) {
  const clean = String(value ?? '').replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '')
  try {
    return JSON.stringify(JSON.parse(clean), null, 2)
  } catch {
    return clean
  }
}

function timeago(value) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''

  const secondsPast = (Date.now() - date.getTime()) / 1000
  if (secondsPast < 0) return ''
  if (secondsPast < 60) return `${Math.floor(secondsPast)}s ago`
  if (secondsPast < 3600) {
    const minutes = Math.floor(secondsPast / 60)
    return `${minutes}${minutes === 1 ? 'min' : 'mins'} ago`
  }
  if (secondsPast <= 86400) return `${Math.floor(secondsPast / 3600)}h ago`
  if (secondsPast <= 2592000) return `${Math.floor(secondsPast / 86400)}d ago`
  if (secondsPast <= 31536000) return `${Math.floor(secondsPast / 2592000)}mth ago`
  return `${Math.floor(secondsPast / 31536000)}yrs ago`
}

function matchesFileQuery(query, fileInfo) {
  const needle = String(query ?? '').trim().toLowerCase()
  if (needle === '') return true

  const filePath = String(fileInfo.file_path ?? '')
  if (fileInfo.type === 'stdin' || filePath.startsWith('/tmp/GOL-')) {
    return false
  }

  const fileName = filePath.split(/[\\/]/).pop()
  return [fileName, filePath].some((field) => String(field ?? '').toLowerCase().includes(needle))
}

function fileSourceLabel(fileInfo) {
  const filePath = String(fileInfo.file_path ?? '')
  if (fileInfo.type === 'file') {
    if (filePath.startsWith('/logs/')) {
      return filePath.split('/')[2] || '/logs'
    }
    return filePath.split('/').slice(0, -1).join('/') || 'Local files'
  }
  if (fileInfo.type === 'ssh') return fileInfo.host || 'SSH'
  if (fileInfo.type === 'docker') return fileInfo.name || fileInfo.host || 'Docker'
  if (fileInfo.type === 'stdin') return 'STDIN'
  return fileInfo.type || 'Files'
}

function defaultResult() {
  return {
    lines: [],
    match_pattern: '',
    total: 0,
    file_path: '',
    host: '',
    source_id: '',
    type: ''
  }
}

export function createGolViewer() {
  return {
    baseURL: getBaseURL(),
    intervalId: null,
    input: {
      query: '',
      query_file: '',
      ignore: '',
      file_path: '',
      realtime: true,
      reverse: true,
      host: '',
      source_id: '',
      type: '',
      page: 1,
      per_page: 100,
      drop_down_search_file: true
    },
    highlighter: {
      line_from: 0,
      line_upto: 0
    },
    results: {
      result: defaultResult(),
      file_paths: [],
      file_paths_backup: []
    },
    loading: {
      fetching: false,
      error: '',
      errorJSON: '',
      updated_at: ''
    },
    groupExpanded: {},
    formatBytes,
    numberToK,
    formatLogString,
    timeago,
    fileGroups() {
      const groups = []
      const seen = new Set()

      for (const fileInfo of this.results.file_paths) {
        const label = fileSourceLabel(fileInfo)
        const key = `${fileInfo.type}:${label}`
        if (seen.has(key)) continue
        seen.add(key)
        groups.push({
          key,
          type: fileInfo.type,
          label,
          count: this.results.file_paths.filter(
            (current) => current.type === fileInfo.type && fileSourceLabel(current) === label
          ).length
        })
      }

      return groups
    },
    groupFiles(group) {
      return this.results.file_paths.filter(
        (fileInfo) => fileInfo.type === group.type && fileSourceLabel(fileInfo) === group.label
      )
    },
    isGroupOpen(group) {
      return this.groupExpanded[group.key] === true
    },
    toggleGroup(group) {
      this.groupExpanded[group.key] = !this.isGroupOpen(group)
    },
    async init() {
      await this.fetchLogs()
    },
    async submit() {
      await this.fetchLogs()
    },
    submitFile() {
      const query = this.input.query_file.trim()
      if (query === '') {
        this.results.file_paths = this.results.file_paths_backup
        return
      }

      document.getElementById('file-groups')?.scroll(0, 0)
      this.results.file_paths = this.results.file_paths_backup.filter((fileInfo) =>
        matchesFileQuery(query, fileInfo)
      )
    },
    async fetchLogs() {
      this.input.host = this.input.host ?? ''
      this.input.source_id = this.input.source_id ?? ''
      this.input.type = this.input.type ?? ''
      this.loading.error = ''
      this.loading.errorJSON = ''
      this.loading.fetching = true

      const params = new URLSearchParams({
        query: this.input.query,
        ignore: this.input.ignore,
        page: this.input.page,
        per_page: this.input.per_page,
        file_path: this.input.file_path,
        host: this.input.host,
        source_id: this.input.source_id,
        type: this.input.type,
        reverse: this.input.reverse
      })

      let response
      try {
        response = await fetch(`${this.baseURL}/api?${params.toString()}`)
      } catch (error) {
        this.loading.fetching = false
        this.loading.error = error instanceof Error ? error.message : String(error)
        return
      }

      this.loading.updated_at = new Date().toLocaleTimeString()
      this.loading.fetching = false

      if (!response.ok) {
        this.loading.error = response.statusText
        try {
          this.loading.errorJSON = JSON.stringify(await response.json(), null, 2)
        } catch {
          this.loading.errorJSON = await response.text()
        }
        return
      }

      const payload = await response.json()
      const result = payload.result ?? defaultResult()
      if (this.input.reverse && result.lines) {
        result.lines = result.lines.reverse()
        this.highlighter.line_from = this.highlighter.line_upto
        this.highlighter.line_upto = result.lines.length > 0 ? result.lines[0].line_number : 0
      }

      if (this.input.query_file === '') {
        this.results.file_paths = payload.file_paths ?? []
      }
      this.results.file_paths_backup = payload.file_paths ?? []
      this.results.result = result
      this.input.file_path = result.file_path ?? ''
      this.input.host = result.host ?? ''
      this.input.source_id = result.source_id ?? ''
      this.input.type = result.type ?? ''

      this.manageRealtimeUpdates()
    },
    manageRealtimeUpdates() {
      if (this.intervalId) {
        clearInterval(this.intervalId)
      }
      if (this.input.realtime) {
        this.intervalId = setInterval(() => this.fetchLogs(), 5000)
      }
    }
  }
}

Alpine.data('golViewer', createGolViewer)
window.Alpine = Alpine
Alpine.start()
