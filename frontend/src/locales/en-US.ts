import { registerMessages } from '../composables/useI18n'

/** Public-facing copy. Admin dashboard copy is migrated in a later pass. */
export const enUS: Record<string, string> = {
  // Site metadata (title / share description)
  'site.statusSuffix': 'Service Status',
  'site.description': '{name} — real-time service availability, SLA history and operational announcements.',

  // Common
  'common.loading': 'Loading...',
  'common.dataRefreshFailed': 'Failed to refresh data',
  'common.loadFailed': 'Failed to load data',
  'common.backToStatus': 'Back to status page',
  'common.close': 'Close',
  'common.copy': 'Copy',
  'common.cst': 'UTC+8',
  'common.notFoundService': 'Service not found. It may have been removed or is no longer listed on the status page.',
  'common.adminPanel': 'Admin',

  // Header / theme menu / language
  'header.subscribe': 'Subscribe',
  'header.switchTheme': 'Switch theme',
  'header.theme.light': 'Light',
  'header.theme.dark': 'Dark',
  'header.theme.system': 'System',
  'header.language': 'Language',

  // Home
  'home.allOperational': 'All systems operational',
  'home.outage': 'Service disruption',
  'home.systemStatus': 'System status',
  'home.maintenances': 'Scheduled maintenance',
  'home.noMaintenance': 'No scheduled maintenance',
  'home.noMaintenanceHint': 'We will announce affected maintenance windows in advance.',
  'home.pastIncidents': 'Past incidents',
  'home.viewServiceDetail': 'View service detail',
  'home.affectedServices': 'Affected services',
  'home.createdAt': 'Created',
  'home.updatedAt': 'Last updated',
  'home.incidentTimeline': 'Incident timeline',
  'home.noUpdates': 'No updates yet',
  'home.invalidIncident': 'Invalid incident identifier',
  'home.loadIncidentFailed': 'Failed to load incident details',
  'home.serviceLabel': 'Service #{id}',
  'home.poweredBy': 'Powered By LumiPulse',

  // Service status
  'status.operational': 'Operational',
  'status.degraded': 'Degraded',
  'status.outage': 'Outage',
  'status.maintenance': 'Under maintenance',
  'status.noData': 'No data',
  'status.healthyToday': 'Operational',
  'status.downtimeLabel': 'Downtime: {duration}',
  'status.daysAgo': '{n} days ago',
  'status.today': 'Today',
  'status.uptimeLabel': '{n}% uptime',

  // Incidents
  'incident.status.investigating': 'Investigating',
  'incident.status.identified': 'Identified',
  'incident.status.monitoring': 'Monitoring',
  'incident.status.resolved': 'Resolved',
  'incident.impact.minor': 'Minor',
  'incident.impact.major': 'Major',
  'incident.impact.critical': 'Critical',

  // Service detail
  'service.uptime': 'Uptime',
  'service.responseTime': 'Response time',
  'service.probeInterval': 'Probe interval',
  'service.latency24h': 'Latency (last 24 hours)',
  'service.latencySamples': '{n} successful samples',
  'service.failures': '{n} failed probes',
  'service.avgLatency': 'Average',
  'service.p95': 'P95',
  'service.p99': 'P99',
  'service.peak': 'Peak',
  'service.noData': 'No data yet',
  'service.history': 'Service history',
  'service.maintenanceOngoing': 'Under maintenance',

  // Latency chart
  'chart.normal': 'Operational',
  'chart.failure': 'Failure',
  'chart.noData': 'No data',

  // Subscribe
  'subscribe.title': 'Subscribe to updates',
  'subscribe.tab.email': 'Email',
  'subscribe.desc': 'Get notified about status changes and incidents.',
  'subscribe.emailPlaceholder': 'Enter your email...',
  'subscribe.submit': 'Subscribe',
  'subscribe.submitting': 'Submitting...',
  'subscribe.success': 'Subscribed! We will email you when service status changes.',
  'subscribe.failed': 'Subscription failed, please try again later',
  'subscribe.selectServices': 'Choose specific services',
  'subscribe.collapse': 'Collapse',
  'subscribe.selectAll': 'Select all',
  'subscribe.clearAll': 'Clear all',
  'subscribe.emptyMeansAll': 'Leave empty to subscribe to all services',
  'subscribe.feedDesc': 'Copy the link below into your RSS reader to follow status updates.',
  'subscribe.copied': 'Link copied to clipboard',

  // Duration
  'duration.minutes': '{n}m',
  'duration.hoursMinutes': '{h}h {m}m',
  'duration.hoursOnly': '{h}h',
}

registerMessages('en-US', enUS)
