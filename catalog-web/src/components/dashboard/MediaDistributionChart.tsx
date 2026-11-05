import { 
  PieChart, 
  Pie, 
  Cell, 
  ResponsiveContainer, 
  Legend, 
  Tooltip
} from 'recharts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { PieChart as PieChartIcon } from 'lucide-react'

interface MediaDistributionData {
  name: string
  value: number
  color: string
}

interface MediaDistributionChartProps {
  data?: Record<string, number>
  loading?: boolean
}

const COLORS = {
  video: '#3b82f6', // blue
  audio: '#10b981', // emerald
  image: '#f59e0b', // amber
  document: '#6b7280', // gray
  other: '#ef4444' // red
}

/**
 * MediaDistributionChart renders a pie chart showing the breakdown of
 * media files by type (video, audio, image, document).
 */
export const MediaDistributionChart: React.FC<MediaDistributionChartProps> = ({
  data,
  loading = false
}) => {
  const formatData = (): MediaDistributionData[] => {
    if (!data) return []
    
    return Object.entries(data).map(([key, value]) => ({
      name: key.charAt(0).toUpperCase() + key.slice(1),
      value,
      color: COLORS[key as keyof typeof COLORS] || COLORS.other
    }))
  }

  const chartData = formatData()

  const CustomTooltip = ({ active, payload }: { active?: boolean; payload?: Array<{ payload: MediaDistributionData }> }) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      const total = chartData.reduce((sum, item) => sum + item.value, 0)
      const percentage = ((data.value / total) * 100).toFixed(1)
      
      return (
        <div className="bg-white p-3 border border-gray-200 rounded-lg shadow-lg">
          <p className="font-semibold text-gray-900">{data.name}</p>
          <p className="text-sm text-gray-600">
            {data.value.toLocaleString()} files ({percentage}%)
          </p>
        </div>
      )
    }
    return null
  }

  const renderCustomizedLabel = ({
    cx, cy, midAngle, innerRadius, outerRadius, percent
  }: { cx: number; cy: number; midAngle: number; innerRadius: number; outerRadius: number; percent: number }) => {
    const RADIAN = Math.PI / 180
    const radius = innerRadius + (outerRadius - innerRadius) * 0.5
    const x = cx + radius * Math.cos(-midAngle * RADIAN)
    const y = cy + radius * Math.sin(-midAngle * RADIAN)

    if (percent < 0.05) return null // Don't show label for small slices

    return (
      <text 
        x={x} 
        y={y} 
        fill="white" 
        textAnchor={x > cx ? 'start' : 'end'} 
        dominantBaseline="central"
        className="text-xs font-medium"
      >
        {`${(percent * 100).toFixed(0)}%`}
      </text>
    )
  }

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <PieChartIcon className="h-5 w-5" />
            Media Distribution
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-80 flex items-center justify-center">
            <div className="animate-pulse space-y-4 w-full">
              <div className="h-4 bg-gray-200 rounded w-1/2 mx-auto"></div>
              <div className="h-48 bg-gray-200 rounded-full mx-auto"></div>
              <div className="flex space-x-4 justify-center">
                <div className="h-3 bg-gray-200 rounded w-16"></div>
                <div className="h-3 bg-gray-200 rounded w-16"></div>
                <div className="h-3 bg-gray-200 rounded w-16"></div>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    )
  }

  if (!chartData || chartData.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <PieChartIcon className="h-5 w-5" />
            Media Distribution
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-80 flex items-center justify-center">
            <div className="text-center">
              <PieChartIcon className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <p className="text-gray-500 dark:text-gray-400">No media data available</p>
              <p className="text-sm text-gray-600 dark:text-gray-400">Media will appear here once scanned</p>
            </div>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <PieChartIcon className="h-5 w-5" />
          Media Distribution
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={320}>
          <PieChart>
            <Pie
              data={chartData}
              cx="50%"
              cy="50%"
              labelLine={false}
              label={renderCustomizedLabel}
              outerRadius={100}
              fill="#8884d8"
              dataKey="value"
            >
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.color} aria-label={`${entry.name}: ${entry.value.toLocaleString()} items`} />
              ))}
            </Pie>
            <Tooltip content={<CustomTooltip />} />
            <Legend 
              formatter={(value: string, entry: { color?: string; payload?: { value?: number } }) => (
                <span style={{ color: entry.color }}>
                  {value}: {entry?.payload?.value?.toLocaleString()}
                </span>
              )}
              verticalAlign="bottom"
              height={36}
            />
          </PieChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}