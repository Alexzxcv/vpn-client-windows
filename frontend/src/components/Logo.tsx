interface LogoProps {
  className?: string;
}

/**
 * Фирменная марка SAPN VPN: щит с маршрут-графом из нод (route motif, акцент ion).
 * Масштабируется; читается от 16px (фавикон) до крупного размера.
 */
export function Logo({ className }: LogoProps) {
  return (
    <svg
      viewBox="0 0 32 32"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      role="img"
      aria-label="SAPN VPN"
    >
      <path
        d="M16 4.2 L26 8 V15.4 C26 21.6 21.7 25.4 16 27.8 C10.3 25.4 6 21.6 6 15.4 V8 Z"
        fill="url(#sapnLogoGrad)"
        stroke="#3DA9FC"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      <path
        d="M16 11.2 L11.4 18.6 M16 11.2 L20.6 18.6 M11.4 18.6 L20.6 18.6"
        stroke="#3DA9FC"
        strokeWidth="1.2"
        strokeLinecap="round"
        opacity="0.5"
      />
      <circle cx="16" cy="11.2" r="2.1" fill="#3DA9FC" />
      <circle cx="11.4" cy="18.6" r="1.6" fill="#9FE0FF" />
      <circle cx="20.6" cy="18.6" r="1.6" fill="#9FE0FF" />
      <defs>
        <linearGradient
          id="sapnLogoGrad"
          x1="16"
          y1="4"
          x2="16"
          y2="28"
          gradientUnits="userSpaceOnUse"
        >
          <stop stopColor="#3DA9FC" stopOpacity="0.20" />
          <stop offset="1" stopColor="#3DA9FC" stopOpacity="0.02" />
        </linearGradient>
      </defs>
    </svg>
  );
}

export default Logo;
