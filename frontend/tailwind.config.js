/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      colors: {
        console: '#0B0F19',
        panel: '#111827',
        surface: '#1E293B',
        steel: '#98A6C8',
        ice: '#00F2FE',
        neon: '#4FACFE',
        violet: '#8B5CF6',
        success: '#34D399',
        danger: '#FB7185',
        amber: '#F59E0B'
      },
      boxShadow: {
        neon: '0 10px 30px rgba(79, 172, 254, 0.28)',
        green: '0 10px 30px rgba(52, 211, 153, 0.28)',
        violet: '0 12px 36px rgba(139, 92, 246, 0.22)',
        glass: '0 8px 32px rgba(0, 0, 0, 0.3)'
      },
      fontFamily: {
        sans: ['Inter', 'PingFang SC', 'Microsoft YaHei', 'system-ui', 'sans-serif']
      },
      keyframes: {
        shimmer: {
          '0%': { backgroundPosition: '200% 0' },
          '100%': { backgroundPosition: '-200% 0' }
        },
        slideIn: {
          '0%': { transform: 'translateY(-20px)', opacity: '0' },
          '100%': { transform: 'translateY(0)', opacity: '1' }
        },
        flowDash: {
          '0%': { strokeDashoffset: '0' },
          '100%': { strokeDashoffset: '-100' }
        }
      },
      animation: {
        shimmer: 'shimmer 2.6s linear infinite',
        slideIn: 'slideIn 0.45s cubic-bezier(0.22, 1, 0.36, 1)',
        flowDash: 'flowDash 4s linear infinite'
      }
    }
  },
  plugins: []
}
