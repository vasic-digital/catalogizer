import React from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { Button } from '../ui/Button'
import { Card, CardContent, CardHeader, CardTitle } from '../ui/Card'
import { useWizard } from '../../contexts/WizardContext'
import { useConfiguration } from '../../contexts/ConfigurationContext'
import { ChevronLeft, ChevronRight, Settings, FileText } from 'lucide-react'

export default function WizardLayout() {
  const { state, nextStep, previousStep, setTotalSteps } = useWizard()
  const { state: configState } = useConfiguration()
  const location = useLocation()
  const navigate = useNavigate()

  // Dynamic steps based on selected protocol
  const getSteps = () => {
    const baseSteps = [
      { path: '/', title: 'Welcome', description: 'Introduction to the setup wizard' },
      { path: '/protocol', title: 'Protocol Selection', description: 'Choose your storage protocol' },
    ]

    if (configState.selectedProtocol === 'smb') {
      baseSteps.push({ path: '/scan', title: 'Network Scan', description: 'Discover hosts on your network' })
    }

    // Add the appropriate configuration step
    if (configState.selectedProtocol) {
      const configStep = {
        path: `/configure-${configState.selectedProtocol}`,
        title: `${configState.selectedProtocol.toUpperCase()} Configuration`,
        description: `Configure ${configState.selectedProtocol.toUpperCase()} connections`
      }
      baseSteps.push(configStep)
    }

    // Add remaining steps
    baseSteps.push(
      { path: '/manage', title: 'Configuration Management', description: 'Manage your configuration' },
      { path: '/summary', title: 'Summary', description: 'Review and finalize setup' }
    )

    return baseSteps
  }

  const steps = getSteps()

  // Update total steps when steps change
  React.useEffect(() => {
    setTotalSteps(steps.length)
  }, [steps.length, setTotalSteps])

  const currentStepIndex = steps.findIndex(step => step.path === location.pathname)
  const currentStep = steps[currentStepIndex] || steps[0]

  const handleNext = () => {
    if (currentStepIndex < steps.length - 1) {
      // Special handling for protocol selection -> configuration step
      if (location.pathname === '/protocol' && configState.selectedProtocol) {
        const configPath = `/configure-${configState.selectedProtocol}`
        navigate(configPath)
        // Skip to the appropriate step index
        const configStepIndex = steps.findIndex(step => step.path === configPath)
        if (configStepIndex !== -1) {
          // This is a bit hacky, but we need to update the wizard state
          for (let i = currentStepIndex + 1; i < configStepIndex; i++) {
            nextStep()
          }
        }
      } else {
        navigate(steps[currentStepIndex + 1].path)
        nextStep()
      }
    }
  }

  const handlePrevious = () => {
    if (currentStepIndex > 0) {
      navigate(steps[currentStepIndex - 1].path)
      previousStep()
    }
  }

  const canNext = state.canGoNext && currentStepIndex < steps.length - 1
  const canPrevious = state.canGoPrevious && currentStepIndex > 0

  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 p-3 pb-10 sm:p-6 sm:pb-10">
      <div className="mx-auto max-w-6xl">
        {/* Header */}
        <div className="mb-8 text-center">
          <div className="flex flex-wrap items-center justify-center gap-2 mb-4">
            <Settings className="h-8 w-8 text-blue-600" />
            <h1 className="text-2xl sm:text-3xl font-bold text-gray-900 break-words">Catalogizer Installation Wizard</h1>
          </div>
          <p className="text-lg text-gray-600">
            Configure storage sources for your media collection
          </p>
        </div>

        {/* Progress Bar */}
        <div className="mb-8">
          <div className="flex items-center justify-between mb-4">
            {steps.map((step, index) => (
              <div
                key={step.path}
                className={`flex items-center ${index < steps.length - 1 ? 'flex-1' : ''}`}
              >
                <div className="flex flex-col items-center">
                  <div
                    className={`w-10 h-10 rounded-full flex items-center justify-center border-2 transition-colors ${
                      index === currentStepIndex
                        ? 'bg-blue-600 border-blue-600 text-white'
                        : index < currentStepIndex
                        ? 'bg-green-600 border-green-600 text-white'
                        : 'bg-white border-gray-300 text-gray-500'
                    }`}
                  >
                    <span className="text-sm font-medium">{index + 1}</span>
                  </div>
                  <div className="mt-2 text-center">
                    <div className={`text-xs sm:text-sm font-medium ${
                      index === currentStepIndex ? 'text-blue-600' : 'text-gray-500'
                    }`}>
                      {step.title}
                    </div>
                  </div>
                </div>
                {index < steps.length - 1 && (
                  <div className={`flex-1 h-1 mx-1 sm:mx-4 rounded ${
                    index < currentStepIndex ? 'bg-green-600' : 'bg-gray-200'
                  }`} />
                )}
              </div>
            ))}
          </div>
        </div>

        {/* Main Content */}
        <Card className="mb-8">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <FileText className="h-6 w-6" />
              {currentStep.title}
            </CardTitle>
            <p className="text-muted-foreground">{currentStep.description}</p>
          </CardHeader>
          <CardContent>
            <Outlet />
          </CardContent>
        </Card>

        {/* Navigation */}
        <div className="flex justify-between items-center">
          <Button
            variant="outline"
            onClick={handlePrevious}
            disabled={!canPrevious}
            className="flex items-center gap-2 min-h-[44px]"
          >
            <ChevronLeft className="h-4 w-4" />
            Previous
          </Button>

          <div className="text-sm text-gray-500">
            Step {currentStepIndex + 1} of {steps.length}
          </div>

          <Button
            onClick={handleNext}
            disabled={!canNext}
            className="flex items-center gap-2 min-h-[44px]"
          >
            {currentStepIndex === steps.length - 1 ? 'Finish' : 'Next'}
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      </div>
      <span className="block mt-4 text-center text-xs text-slate-500 select-none">v1.1.0</span>
    </div>
  )
}