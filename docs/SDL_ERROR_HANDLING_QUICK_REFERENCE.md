# SDL Error Handling Quick Reference

## 🚀 **Basic Try/Catch Syntax**

```yaml
- name: task_name
  type: try
  config:
    parameters:
      try:
        # Tasks to execute
        taskName:
          task_type: call
          config:
            parameters:
              # Task configuration
      catch:
        # Error handling configuration
        errors:
          with:
            # Error filter criteria
        retry:
          # Retry policy
        do:
          # Recovery tasks
```

## 🔄 **Retry Policies**

### Constant Backoff
```yaml
retry:
  delay:
    seconds: 2
  backoff:
    constant: {}
  limit:
    attempt:
      count: 5
```

### Linear Backoff
```yaml
retry:
  delay:
    seconds: 1
  backoff:
    linear:
      increment:
        seconds: 1
  limit:
    attempt:
      count: 5
```

### Exponential Backoff
```yaml
retry:
  delay:
    milliseconds: 500
  backoff:
    exponential:
      multiplier: 2.0
      maxDelay:
        seconds: 30
  jitter:
    from:
      milliseconds: 100
    to:
      milliseconds: 500
  limit:
    attempt:
      count: 10
    duration:
      minutes: 5
```

## 🎯 **Error Filtering**

### By HTTP Status
```yaml
errors:
  with:
    status: 503
```

### By Error Type
```yaml
errors:
  with:
    type: "https://serverlessworkflow.io/spec/1.0.0/errors/communication"
```

### Multiple Criteria
```yaml
errors:
  with:
    type: "https://serverlessworkflow.io/spec/1.0.0/errors/communication"
    status: 503
```

### Pattern Matching
```yaml
errors:
  with:
    type: "https://serverlessworkflow.io/spec/1.0.0/errors/(communication|timeout)"
```

## 🛠️ **Error Recovery**

### Simple Recovery
```yaml
catch:
  errors:
    with:
      status: 402
  as: "paymentError"
  do:
    notifyUser:
      task_type: call
      config:
        parameters:
          call: http
          with:
            method: post
            endpoint: https://notification.service.com/send
            body:
              message: "Error: ${.paymentError.detail}"
```

### Multiple Recovery Tasks
```yaml
catch:
  errors:
    with:
      status: 500
  as: "systemError"
  do:
    logError:
      task_type: call
      config:
        parameters:
          call: http
          with:
            method: post
            endpoint: https://logging.service.com/log
    alertOps:
      task_type: call
      config:
        parameters:
          call: http
          with:
            method: post
            endpoint: https://alerts.service.com/alert
```

## 📊 **Status Tracking**

### Task Status Values
- `TASK_STATUS_PENDING`: Task queued for execution
- `TASK_STATUS_RUNNING`: Task currently executing
- `TASK_STATUS_COMPLETED`: Task finished successfully
- `TASK_STATUS_FAILED`: Task failed after retries exhausted
- `TASK_STATUS_CANCELLED`: Task cancelled by user
- `TASK_STATUS_RETRYING`: Task currently retrying

### Workflow Status Values
- `WORKFLOW_STATUS_RUNNING`: Workflow executing normally
- `WORKFLOW_STATUS_FAILED`: Workflow failed due to task failure
- `WORKFLOW_STATUS_COMPLETED`: Workflow finished successfully

### Error Information Storage
When a task fails, error information is automatically stored in workflow variables:
- `error_message`: The actual error message
- `failed_task`: Name of the task that failed

## 🎨 **Common Patterns**

### API Call with Exponential Backoff
```yaml
- name: api_call_resilient
  type: try
  config:
    parameters:
      try:
        callAPI:
          task_type: call
          config:
            parameters:
              call: http
              with:
                method: get
                endpoint: https://api.example.com/data
                headers:
                  Authorization: "Bearer ${.token}"
      catch:
        errors:
          with:
            type: "https://serverlessworkflow.io/spec/1.0.0/errors/communication"
        retry:
          delay:
            seconds: 1
          backoff:
            exponential:
              multiplier: 2.0
              maxDelay:
                seconds: 60
          jitter:
            from:
              milliseconds: 100
            to:
              milliseconds: 500
          limit:
            attempt:
              count: 5
```

### Database Operation with Recovery
```yaml
- name: database_operation
  type: try
  config:
    parameters:
      try:
        updateRecord:
          task_type: call
          config:
            parameters:
              call: http
              with:
                method: put
                endpoint: https://db.service.com/records/${.recordId}
                body:
                  data: "${.updateData}"
      catch:
        errors:
          with:
            status: 409  # Conflict
        as: "conflictError"
        do:
          refreshAndRetry:
            task_type: call
            config:
              parameters:
                call: http
                with:
                  method: get
                  endpoint: https://db.service.com/records/${.recordId}
```

### Payment Processing with Fallback
```yaml
- name: payment_processing
  type: try
  config:
    parameters:
      try:
        primaryPayment:
          task_type: call
          config:
            parameters:
              call: http
              with:
                method: post
                endpoint: https://primary-payment.service.com/charge
                body:
                  amount: "${.order.total}"
                  currency: "USD"
      catch:
        errors:
          with:
            status: 503
        as: "primaryPaymentError"
        do:
          fallbackPayment:
            task_type: call
            config:
              parameters:
                call: http
                with:
                  method: post
                  endpoint: https://backup-payment.service.com/charge
                  body:
                    amount: "${.order.total}"
                    currency: "USD"
                    reason: "Primary payment failed: ${.primaryPaymentError.detail}"
```

## 🔧 **Best Practices**

1. **Use Appropriate Backoff**: Exponential backoff for network errors, constant for validation errors
2. **Set Reasonable Limits**: Don't retry indefinitely - set both attempt and duration limits
3. **Add Jitter**: Prevent thundering herd problems with random jitter
4. **Filter Errors Carefully**: Only retry errors that are likely to succeed on retry
5. **Provide Recovery**: Always have a plan for when retries are exhausted
6. **Monitor and Alert**: Use error information for monitoring and alerting

## 📚 **Error Types Reference**

### SDL Standard Error Types
- `https://serverlessworkflow.io/spec/1.0.0/errors/communication`: Network/HTTP errors
- `https://serverlessworkflow.io/spec/1.0.0/errors/timeout`: Timeout errors
- `https://serverlessworkflow.io/spec/1.0.0/errors/validation`: Input validation errors
- `https://serverlessworkflow.io/spec/1.0.0/errors/authentication`: Authentication errors
- `https://serverlessworkflow.io/spec/1.0.0/errors/authorization`: Authorization errors
- `https://serverlessworkflow.io/spec/1.0.0/errors/configuration`: Configuration errors
- `https://serverlessworkflow.io/spec/1.0.0/errors/runtime`: Runtime execution errors

### HTTP Status Code Ranges
- `4xx`: Client errors (usually don't retry)
- `5xx`: Server errors (good candidates for retry)
- `503`: Service Unavailable (excellent retry candidate)
- `429`: Rate Limited (retry with backoff)
- `408`: Request Timeout (retry candidate)
