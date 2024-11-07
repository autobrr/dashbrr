export class NotificationService {
  private static instance: NotificationService;
  private permission: NotificationPermission = 'default';
  private enabled: boolean = true;

  private constructor() {
    this.permission = Notification.permission;
  }

  public static getInstance(): NotificationService {
    if (!NotificationService.instance) {
      NotificationService.instance = new NotificationService();
    }
    return NotificationService.instance;
  }

  public async requestPermission(): Promise<boolean> {
    if (!('Notification' in window)) {
      console.warn('This browser does not support notifications');
      return false;
    }

    try {
      const permission = await Notification.requestPermission();
      this.permission = permission;
      return permission === 'granted';
    } catch (error) {
      console.error('Error requesting notification permission:', error);
      return false;
    }
  }

  public isEnabled(): boolean {
    return this.enabled;
  }

  public setEnabled(enabled: boolean): void {
    this.enabled = enabled;
  }

  public async notify(title: string, options?: NotificationOptions): Promise<boolean> {
    if (!this.enabled || !('Notification' in window)) {
      return false;
    }

    if (this.permission !== 'granted') {
      const granted = await this.requestPermission();
      if (!granted) {
        return false;
      }
    }

    try {
      new Notification(title, {
        icon: '/pwa-192x192.png',
        ...options
      });
      return true;
    } catch (error) {
      console.error('Error showing notification:', error);
      return false;
    }
  }

  public async notifyServiceUpdate(
    serviceName: string, 
    status: string, 
    message?: string
  ): Promise<boolean> {
    return this.notify(
      `${serviceName} Status Update`,
      {
        body: message || `Status changed to: ${status}`,
        tag: `service-${serviceName}-status`
      }
    );
  }

  public async notifyVersionUpdate(
    serviceName: string,
    currentVersion: string,
    newVersion: string
  ): Promise<boolean> {
    return this.notify(
      `${serviceName} Update Available`,
      {
        body: `New version available: ${newVersion} (current: ${currentVersion})`,
        tag: `service-${serviceName}-version`
      }
    );
  }
}

export default NotificationService.getInstance();
