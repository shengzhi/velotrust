import React from 'react';
import { QRCodeSVG } from 'qrcode.react';

interface QrCodeSvgProps {
  value: string;
  size?: number;
  className?: string;
}

/** Render a standards-compliant QR code as an SVG. */
export const QrCodeSvg: React.FC<QrCodeSvgProps> = ({ value, size = 180, className = '' }) => (
  <QRCodeSVG
    value={value}
    size={size}
    level="M"
    includeMargin
    title="二维码"
    className={className}
  />
);
