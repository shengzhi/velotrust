/**
 * Formats a byte number into human-readable string (e.g. 1.25 GB)
 */
export function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  if (i < 0) return `${bytes} B`;
  const index = Math.min(i, sizes.length - 1);
  return `${parseFloat((bytes / Math.pow(k, index)).toFixed(dm))} ${sizes[index]}`;
}

/**
 * Returns a Chinese relative time string (e.g. 刚刚, 5分钟前, 2小时前, 3天前)
 */
export function formatRelativeTime(dateInput: string | number | Date): string {
  if (!dateInput) return '-';
  const now = new Date().getTime();
  const date = new Date(dateInput).getTime();
  const diffInSeconds = Math.floor((now - date) / 1000);

  if (diffInSeconds < 0 || diffInSeconds < 30) {
    return '刚刚';
  }
  if (diffInSeconds < 60) {
    return `${diffInSeconds} 秒前`;
  }
  const diffInMinutes = Math.floor(diffInSeconds / 60);
  if (diffInMinutes < 60) {
    return `${diffInMinutes} 分钟前`;
  }
  const diffInHours = Math.floor(diffInMinutes / 60);
  if (diffInHours < 24) {
    return `${diffInHours} 小时前`;
  }
  const diffInDays = Math.floor(diffInHours / 24);
  if (diffInDays < 30) {
    return `${diffInDays} 天前`;
  }
  const diffInMonths = Math.floor(diffInDays / 30);
  if (diffInMonths < 12) {
    return `${diffInMonths} 个月前`;
  }
  return `${Math.floor(diffInDays / 365)} 年前`;
}

/**
 * Formats full date time string: YYYY-MM-DD HH:mm:ss
 */
export function formatDateTime(dateInput: string | number | Date): string {
  if (!dateInput) return '-';
  const d = new Date(dateInput);
  if (isNaN(d.getTime())) return String(dateInput);

  const pad = (n: number) => (n < 10 ? `0${n}` : n);
  const year = d.getFullYear();
  const month = pad(d.getMonth() + 1);
  const day = pad(d.getDate());
  const hours = pad(d.getHours());
  const minutes = pad(d.getMinutes());
  const seconds = pad(d.getSeconds());

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
}

/**
 * Formats IP and Port
 */
export function formatIpPort(ip: string, port: number | string): string {
  if (!ip) return '-';
  return `${ip}:${port}`;
}

/**
 * Validates IPv4 CIDR notation (e.g. 192.168.1.0/24)
 */
export function isValidCidr(cidr: string): boolean {
  const cidrRegex = /^([0-9]{1,3}\.){3}[0-9]{1,3}\/([0-9]|[1-2][0-9]|3[0-2])$/;
  if (!cidrRegex.test(cidr)) return false;
  const [ip, prefix] = cidr.split('/');
  const octets = ip.split('.').map(Number);
  const validOctets = octets.every((o) => o >= 0 && o <= 255);
  const validPrefix = Number(prefix) >= 0 && Number(prefix) <= 32;
  return validOctets && validPrefix;
}

/**
 * Basic email validator
 */
export function isValidEmail(email: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}
