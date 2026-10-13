# Source documentation audit

[Documentation map](index.md) · [Repository README](../README.md)
[Next: API and lifecycle reference](reference.md)

## Outcome, owner, consumers, named risks and smallest checks

Base 5628c7788a8af8dc12b3e56cded324d99f75db54; freshly verified remote-main detached worktree. Tier A documentation-only.
Root maintainers own validated workflow definitions, immutable history/replay,
transition and durable-work identities, processors, workers and operator contracts.
The PostgreSQL package owns its pgx storage/transaction/schema translation.
Applications own business effects, authorization, tenant policy, idempotency,
publication/acknowledgement, pool/transaction lifetime, supervision and recovery.
Direct consumers include the PostgreSQL store, injected stores/handlers/creators,
the existing executable recipes, optional outbox/broker/CloudEvents compositions
and retained v1/v2/v3 migration/compatibility evidence.

Named risks: definition/history/fingerprint/sequence identity; replay and race
winner ordering; unknown external/commit receipt; activity/child/compensation
attempt admission and retries; lease/fencing/clock/worker cancellation and join;
atomic transition/history/work/outbox staging; public mutable inputs versus copied
immutable values; callback/privacy/serialization/allocation and SQL integer bounds.

Smallest checks: audit all 103 tracked Go files/one module/two packages, public
members and important private invariants; source-backed meaningful contracts;
affected formatting and all-source AST/noncomment token/literal/directive proof
(report grammar-optional formatter semicolons only); parsed HTML/native package
renders; configured producer/API preservation, existing example integrity; native
source/localdocs/offline specification and affected links; pinned full/changed/
staged secrets and one ordinary complete final-diff truth/usefulness review.
All Go/native workflows await the coordinator slot with three owned disposable
caches and immediate failure/success cleanup. No runtime repair, normative-history
rewrite, duplicate example, race/fuzz/benchmark/soak/backend/consumer/release campaign.

## Missing versus requiring technical investigation

Missing: expanded canonical package contracts and public input/configuration/
result/interface method documentation. Retain meaningful existing value/type
comments. Investigation required before truthful wording: copying before admission,
callback/clock/error disclosure, lifetime and cancellation cooperation, replay/
sequence/fence/transaction crash windows, monotonic/signed SQL bounds, recovery
and possible maintained prose contradictions. No rationale or conformance is guessed.

## Reproducible topology, native producers, markers and every tracked Go file

Inventory recipe: git ls-files for Go/AGENTS/module/lock files; modules.json and
packages.json; .golib.yaml plus Make/verification/scripts/CI; declaration, helper,
test/fuzz/benchmark/example, directive/generated/deprecated/marker scans.
One module github.com/faustbrian/go-workflow/v3; packages workflow and postgres.
Tracked go.mod/go.sum minimum1.27.0; repository pin1.27.2. postgres is not a
nested module. No generated source/go:generate producer or source TODO/FIXME/HACK/
NOTE/WARNING/deprecation-comment marker found. Seven PostgreSQL integration-tag
files are preserved; no production platform variant or TestMain found.

Native API mode is apidiff, api/v3-baseline.txt, selected by .golib.yaml and the
Tools API producer. No full-prose GoDoc generation script exists. scripts/check-docs.sh
compiles the README quickstart and runs two existing Examples. That configured
operation runs once as the goal's executable-documentation proof; there is no
separate duplicate example run. Candidate/public interoperability,
service-aware Make/CI and 10000x fuzz operations are not prose verification.
Current immutable Tools CI identities and all archived migration/threat evidence
remain untouched; parent owns enforcement/publication.

| Tracked Go file | Package | Role/retained evidence |
| --- | --- | --- |
| `activity.go` | workflow | Source API and private invariants |
| `activity_fuzz_test.go` | workflow_test | Existing source evidence: FuzzActivityRequestValidation, FuzzActivityOutcomeValidation |
| `activity_history.go` | workflow | Source API and private invariants |
| `activity_history_test.go` | workflow_test | Existing source evidence: TestReplayReconstructsPersistedActivityAttemptsAndRetry, TestReplayPreservesKnownFailureAndUnknownActivityOutcomes, TestHistoryActivityEventOwnsCanonicalAttemptMetadata, TestHistoryActivityEventRejectsAmbiguousFields, TestReplayRejectsIllegalActivityTransitions, TestReplayAcceptsExactActivityPayloadLimits, TestReplayEnforcesActivityRetryPolicyAndDueTime, TestReplayDoublesActivityRetryDelayBeforeTheCap, TestReplayPreservesRetryableFailureWhenAttemptsAreExhausted |
| `activity_processor.go` | workflow | Source API and private invariants |
| `activity_processor_internal_test.go` | workflow | Existing source evidence: TestActivityProcessorCommitReconciliationBoundaries, TestActivityProcessorRejectsInvalidConstructionAndCalls, TestActivityProcessorLoadsTheExactMaximumAttempt, TestExecuteActivitySafelyClassifiesExecutionErrorsAsUnknown, TestActivityProcessorLoadedStateDecisions, TestActivityProcessorClassifiesLoadAndStateFailures, TestActivityProcessorPersistenceFailureBoundaries |
| `activity_processor_test.go` | workflow_test | Existing source evidence: TestActivityWorkProcessorPersistsStartBeforeExecutingAndPersistsOutcome, TestActivityWorkProcessorDoesNotRepeatInFlightUnknownSideEffect, TestActivityWorkProcessorSchedulesDurableRetryAfterKnownFailure, TestActivityWorkProcessorPersistsUnknownAfterHandlerPanic, TestActivityWorkProcessorDeadLettersPoisonDispatchWithoutSideEffects |
| `activity_test.go` | workflow_test | Existing source evidence: TestActivityRequestOwnsBoundedAttemptMetadata, TestActivityRequestRejectsMissingOrUnboundedMetadata, TestActivityRequestAcceptsExactAttemptAndInputBounds, TestActivityOutcomeDistinguishesKnownFailureAndUnknownOutcome, TestActivityOutcomeRejectsAmbiguousClassification, TestActivityOutcomeAcceptsExactPayloadLimit, TestActivityExecutesExplicitHandlerWithBoundedContext, TestActivityRejectsInvalidHandlerOrOutcome, TestActivityRegistryRejectsDuplicatesAndResolvesExplicitly |
| `activity_transition.go` | workflow | Source API and private invariants |
| `activity_transition_fuzz_test.go` | workflow_test | Existing source evidence: FuzzActivityDispatchBoundaries |
| `activity_transition_internal_test.go` | workflow | Existing source evidence: TestActivityTransitionInternalFailureBoundaries |
| `activity_transition_test.go` | workflow_test | Existing source evidence: TestActivityTransitionsPersistBeforeDispatchAndExternalExecution, TestActivityOutcomeAndRetryPreserveSemanticAttemptIdentity, TestActivityTransitionBuildersRejectInvalidStateAndBoundaries |
| `administration.go` | workflow | Source API and private invariants |
| `administration_internal_test.go` | workflow | Existing source evidence: TestInstanceListCursorValiditySeparatesInitialAndContinuationCursors, TestInstanceListCursorOrderingSeparatesTimeAndIdentityBoundaries, TestInstanceListQueryAcceptsTheExactPageLimit |
| `administration_test.go` | workflow_test | Existing source evidence: TestInstanceListPageUsesStableCreationCursor, TestInstanceListCursorCanBeReconstructedAtAPIBoundary, TestInstanceAdministrationValuesRejectInvalidAdapterData, TestTransitionReconciliationIdentityIsExplicit |
| `admission_copy_test.go` | workflow_test | Existing source evidence: TestConstructorsRejectBeforeDefensiveCopies |
| `child_fuzz_test.go` | workflow_test | Existing source evidence: FuzzChildDispatchBoundaries, FuzzChildStartRequestBoundaries |
| `child_history.go` | workflow | Source API and private invariants |
| `child_internal_test.go` | workflow | Existing source evidence: TestChildBuildersRejectMalformedDispatchSchedulesAndOutcomes, TestChildReplayAndOrchestrationPreserveKnownFailure, TestChildHistoryRejectsIncoherentTransitions, TestChildEventFieldValidationIsExactForEveryEventKind, TestChildStartHistoryAndBuildersRejectIncoherentBoundaries, TestChildStartAttemptValidatesEachFencedBoundary |
| `child_processor.go` | workflow | Source API and private invariants |
| `child_processor_internal_test.go` | workflow | Existing source evidence: TestChildProcessorRejectsInvalidConstructionCallsAndLoadedStates, TestChildProcessorTreatsPreInvocationCancellationAsKnownAbsent, TestChildProcessorLoadAndPersistenceFailuresRemainFenced, TestChildProcessorCoversPoisonAndStoreFailureBoundaries |
| `child_processor_test.go` | workflow_test | Existing source evidence: TestChildWorkProcessorPersistsStartBeforeCreatingPinnedChild, TestChildWorkProcessorSchedulesRetryOnlyAfterKnownAbsentFailure, TestChildWorkProcessorPersistsUnknownForPanicAndInvalidOutcome, TestChildWorkProcessorDeadLettersPoisonDispatch, TestChildWorkProcessorDoesNotRepeatInFlightUnknownStart |
| `child_start.go` | workflow | Source API and private invariants |
| `child_start_internal_test.go` | workflow | Existing source evidence: TestChildStartValuesRejectInvalidAndOwnInput |
| `child_transition.go` | workflow | Source API and private invariants |
| `child_transition_test.go` | workflow_test | Existing source evidence: TestChildWorkflowScheduleAndOutcomeRemainVersionPinned, TestRegistryValidatesChildDefinitionsAfterNonChildSteps |
| `compensation_fuzz_test.go` | workflow_test | Existing source evidence: FuzzCompensationDispatchBoundaries |
| `compensation_history.go` | workflow | Source API and private invariants |
| `compensation_history_internal_test.go` | workflow | Existing source evidence: TestCompensationHelpersRejectNonCompensationAndRetainScheduleOrder, TestCompensationTransitionModelRejectsEveryInvalidStateBoundary |
| `compensation_history_test.go` | workflow_test | Existing source evidence: TestReplayReconstructsIndependentlyRetryableCompensation, TestReplayPreservesUnknownCompensationForManualResolution, TestHistoryRejectsInvalidCompensationEventFields, TestManualCompensationResolutionNeverReportsSuccessfulRollback, TestReplayRejectsCompensationWithoutSuccessfulActivityOrValidOrder |
| `compensation_processor.go` | workflow | Source API and private invariants |
| `compensation_processor_internal_test.go` | workflow | Existing source evidence: TestCompensationProcessorRejectsInvalidConstructionAndCalls, TestExecuteCompensationSafelyClassifiesExecutionErrorsAsUnknown, TestCompensationProcessorLoadedStateDecisions, TestCompensationProcessorClassifiesLoadAndStateFailures, TestCompensationProcessorAcceptsExactAttemptBoundaries, TestCompensationProcessorPersistenceFailureBoundaries |
| `compensation_processor_test.go` | workflow_test | Existing source evidence: TestCompensationWorkProcessorPersistsStartBeforeExecutingAndOutcomeAfter, TestCompensationWorkProcessorDoesNotRepeatInFlightRollback, TestCompensationWorkProcessorSchedulesIndependentRetryAfterKnownFailure, TestCompensationWorkProcessorPersistsUnknownAfterHandlerPanic, TestCompensationWorkProcessorDeadLettersPoisonDispatch |
| `compensation_transition.go` | workflow | Source API and private invariants |
| `compensation_transition_internal_test.go` | workflow | Existing source evidence: TestCompensationTransitionInternalBoundaries |
| `compensation_transition_test.go` | workflow_test | Existing source evidence: TestCompensationTransitionsPersistIndependentRetryLifecycle, TestCompensationBuildersRejectMalformedOrUnboundedInput |
| `deadletter.go` | workflow | Source API and private invariants |
| `deadletter_internal_test.go` | workflow | Existing source evidence: TestDeadLetterResolutionValidityRequiresFieldsAndExactFingerprint |
| `deadletter_test.go` | workflow_test | Existing source evidence: TestDeadLetterResolutionIsAuditedFencedAndIdempotent, TestDeadLetterPageUsesStableFailureCursor, TestDeadLetterValuesRejectInvalidAndUnboundedInput |
| `definition.go` | workflow | Source API and private invariants |
| `definition_test.go` | workflow_test | Existing source evidence: TestDefinitionOwnsImmutableVersionedBehavior, TestDefinitionSupportsExplicitBoundedStepKinds, TestDefinitionAcceptsEveryExactSafetyBoundary, TestDefinitionRejectsUnsafeOrAmbiguousSteps, TestDefinitionRejectsAmbiguousControlFlowBranches, TestDefinitionRejectsEachChildAndFanOutBoundaryIndependently, TestDefinitionRejectsUnpinnedOrMisplacedChildReferences, TestRegistryPinsVersionsAndRejectsDuplicateRegistration, TestRegistryRequiresExplicitDefinitionMigration, TestRegistryRejectsInvalidDefinitionsAndMigrationEdges |
| `durable_compensation_example_test.go` | workflow_test | Existing source evidence: Example_durableCompensationRecovery |
| `example_test.go` | workflow_test | Existing source evidence: Example_durableOrchestration |
| `hardening_test.go` | workflow_test | Existing source evidence: TestWorkerStressCompletesEveryLeaseWithinItsConcurrencyBound, TestWorkerShutdownJoinsEveryOwnedProcessorGoroutine, TestContinueAsNewSoakKeepsEachHistoryBoundedAndDeterministic |
| `history.go` | workflow | Source API and private invariants |
| `history_fuzz_test.go` | workflow_test | Existing source evidence: FuzzHistoryEventValidation, FuzzActivityHistoryEventValidation |
| `history_test.go` | workflow_test | Existing source evidence: TestReplayDeterministicallyReconstructsPersistedLifecycle, TestReplayRejectsUnpinnedOrConflictingHistory, TestReplayEnforcesLifecycleStateTransitions, TestReplayUsesPersistedMigrationDecisionWithoutReexecutingCode, TestReplayContinueAsNewClosesHistoryWithExplicitSuccessor, TestHistoryEventRejectsMalformedDurableRecords, TestHistoryEventOwnsPersistedDataAndExposesCanonicalFields, TestHistoryEventAcceptsExactPayloadLimit, TestDefinitionReferenceRejectsMalformedIdentity |
| `inspection.go` | workflow | Source API and private invariants |
| `inspection_internal_test.go` | workflow | Existing source evidence: TestHistoryTraversalValidationSeparatesEveryBound, TestHistoryTraversalRejectsAValidPageWithTheWrongCursor |
| `inspection_test.go` | workflow_test | Existing source evidence: TestInspectInstanceReplaysBoundedStableHistoryPages, TestExportHistoryStreamsBoundedOwnedPages, TestInspectionAndExportRejectInvalidOrUnboundedRequests, TestExportHistoryDoesNotInvokeSinkForEmptyHistory |
| `lifecycle_model_test.go` | workflow_test | Existing source evidence: TestLifecycleTransitionModelExhaustivelyMatchesReplay |
| `operator.go` | workflow | Source API and private invariants |
| `operator_fuzz_test.go` | workflow_test | Existing source evidence: FuzzOperatorAuditBoundaries |
| `operator_internal_test.go` | workflow | Existing source evidence: TestOperatorInternalsRejectCorruptAuditAndSequenceOverflow |
| `operator_test.go` | workflow_test | Existing source evidence: TestOperatorLifecycleCommandPersistsAuditBeforeAction, TestOperatorLifecycleCommandsEnforceCurrentStateAndBoundedAudit, TestOperatorAuditEventRejectsMalformedOrUnboundedFields, TestReplayRejectsOrphanedOrMismatchedOperatorAudit |
| `operator_work.go` | workflow | Source API and private invariants |
| `operator_work_internal_test.go` | workflow | Existing source evidence: TestOperatorWorkCommandsRejectInvalidAuditAndActionBoundaries, TestOperatorWorkHelpersRejectInvalidAuditDocumentAndTransition, TestOperatorWorkAuditValidatesEachBoundaryIndependently, TestOperatorCompensationResolutionValidatesEachBoundaryIndependently, TestOperatorApprovalAcceptsTheExactPayloadLimit |
| `operator_work_test.go` | workflow_test | Existing source evidence: TestOperatorActivityRetryIsAuditedAndAtomicallyCreatesWork, TestOperatorCompensationScheduleIsAuditedBeforeDueWork, TestOperatorCompensationResolutionRemainsDistinctFromSuccessfulRollback |
| `orchestration.go` | workflow | Source API and private invariants |
| `orchestration_internal_test.go` | workflow | Existing source evidence: TestOrchestrationRejectsUnsupportedAndInvalidStepDecisions, TestOrchestrationTerminalRejectsInvalidEventAndTransition, TestOrchestrationValidatesEachInstancePrecondition, TestOrchestrationFailureRequiresRetryabilityAndRemainingAttempts, TestParallelSchedulingValidatesEachBranchBoundary, TestOrchestrationBranchMembershipOnlyIncludesParallelAndRaceBranches, TestJoinWaitsForMissingAndNonSuccessfulBranches |
| `orchestration_parallel_internal_test.go` | workflow | Existing source evidence: TestParallelOrchestrationClassifiesEveryPersistedBranchState, TestParallelScheduleRejectsDuplicateMalformedAndUnboundedPlans |
| `orchestration_parallel_test.go` | workflow_test | Existing source evidence: TestOrchestrationSchedulesBoundedParallelActivitiesAndJoinsTheirOutcomes |
| `orchestration_race_test.go` | workflow_test | Existing source evidence: TestOrchestrationPersistsTheFirstObservedRaceWinner, TestReplayRejectsRaceWinnerWithoutObservedBranch |
| `orchestration_test.go` | workflow_test | Existing source evidence: TestOrchestrationDecisionAdvancesPersistedSequentialSteps, TestOrchestrationDecisionFailsKnownTerminalActivityFailure, TestOrchestrationDecisionRejectsChoreographyDefinitions, TestOrchestrationApprovalRequiresAuditedOperatorAcceptance |
| `performance_benchmark_test.go` | workflow_test | Existing source evidence: BenchmarkReplayPersistedLifecycle, BenchmarkPlanDurableActivityTransition |
| `postgres/administration_store_test.go` | postgres | Existing source evidence: TestListInstancesSupportsArchiveSelectionsAndStableCursor, TestListInstancesRejectsFailuresAndCorruptRows, TestReconcileTransitionRejectsInvalidAndDatabaseFailure |
| `postgres/deadletter_store_test.go` | postgres | Existing source evidence: TestListDeadLettersReturnsStableUnresolvedPages, TestListDeadLettersRejectsInvalidFailuresAndCorruptRows, TestScanDeadLetterRecordRejectsDriverAndDurableCorruption, TestResolveDeadLetterClassifiesValidationAndTransactionFailures, TestResolveDeadLetterHandlesReplayConflictAndPersistenceBoundaries |
| `postgres/deadlock_integration_test.go` | postgres | Existing source evidence: TestPostgreSQLDeadlockAbortsOneCompleteTransitionSet; integration build variant |
| `postgres/durable_compensation_recipe_integration_test.go` | postgres | Existing source evidence: TestPostgreSQLDurableCompensationRecipeSurvivesRestartAndDrainsWorker; integration build variant |
| `postgres/failover_integration_test.go` | postgres | Existing source evidence: TestPostgreSQLReplicaPromotionReconcilesAnInterruptedTransition; integration build variant |
| `postgres/integration_test.go` | postgres | Existing source evidence: TestPostgreSQLAtomicTransitionsAndStableHistory, TestPostgreSQLDeadLetterResolutionIsAuditedAndFenced, TestPostgreSQLStageComposesWithCallerOwnedTransaction; integration build variant |
| `postgres/migration.go` | postgres | Source API and private invariants |
| `postgres/pgx_time_integration_test.go` | postgres | Existing source evidence: TestPostgreSQLProtocolModesPreserveWorkflowInstants; integration build variant |
| `postgres/process_death_integration_test.go` | postgres | Existing source evidence: TestPostgreSQLRecoversFromProcessDeathAtDurableBoundaries, TestPostgreSQLProcessDeathAfterActivitySideEffectPreservesUnknownOutcome, TestPostgreSQLProcessDeathHelper; integration build variant |
| `postgres/recovery_integration_test.go` | postgres | Existing source evidence: TestPostgreSQLRestartAndSnapshotRestorePreserveAuthoritativeHistory; integration build variant |
| `postgres/recovery_readiness_internal_test.go` | postgres | Existing source evidence: TestRecoveryReadyRetriesOnlyStartingUp |
| `postgres/store.go` | postgres | Source API and private invariants |
| `postgres/store_boundaries_test.go` | postgres | Existing source evidence: TestSchemaMigrationUsesDefaultSchema, TestStoreRejectsSequencesOutsidePostgreSQLBigint, TestCommitHandlesConcurrentTransitionDecisions, TestCommitRejectsRemainingAtomicWriteFailures, TestCommitPersistsMigrationIdentityAndDueTimes, TestTerminalArchiveTimeMatchesEveryTerminalOutcome, TestHistoryRejectsInvalidRequestsAndDatabaseFailures, TestHistoryRejectsSequenceBeyondPostgreSQLBigint, TestHistoryRejectsCorruptRowsAndPagination, TestScanHistoryEventRejectsExactNumericAndPartialDefinitionBoundaries, TestScanHistoryEventAcceptsExactPersistedNumericBoundaries, TestScanHistoryEventAcceptsRaceAndChildHistoryAddedAfterOperatorCommands, TestHistoryDecodesDefinitionAndDueTime, TestOperationErrorPreservesCauseWithoutExposingIt, TestWorkStoreRequestAndDriverFailureBoundaries, TestFencedMutationDriverFailuresRemainErrors, TestClaimRejectsCorruptDurableWork |
| `postgres/store_fuzz_test.go` | postgres | Existing source evidence: FuzzScanHistoryEvent |
| `postgres/store_test.go` | postgres | Existing source evidence: TestNewRejectsMissingPoolAndUnsafeSchema, TestSchemaMigrationIsVersionedAndOwnsAtomicStoreTables, TestCommitAtomicallyPersistsHistoryAndDueWork, TestCommitTreatsExactTransitionReplayAsIdempotent, TestCommitClassifiesValidationAndTransactionFailures, TestStageRejectsInvalidCallerOwnedTransactionRequests, TestCommitRejectsConflictingTransitionIdentity, TestCommitExistingInstanceUsesOptimisticDefinitionAndSequence, TestCommitFailureBoundariesRemainNotCommittedUntilCommit, TestCommitFailureAfterCommitAttemptIsUnknown, TestHistoryReturnsStableValidatedPages, TestHistoryDoesNotClaimMoreForAnExactlyFullPage, TestListInstancesReturnsStableValidatedCreationPages, TestReconcileTransitionDistinguishesMissingExactAndConflict, TestClaimReturnsStableFencedDueWork, TestRenewCompleteAndFailRejectStaleFences |
| `race_history.go` | workflow | Source API and private invariants |
| `race_history_internal_test.go` | workflow | Existing source evidence: TestRaceHistoryRejectsUnobservedMismatchedAndDuplicateWinners, TestRaceDecisionAndEventRejectMalformedPersistencePlans |
| `registry.go` | workflow | Source API and private invariants |
| `soak_test.go` | workflow_test | Existing source evidence: TestWorkflowAcceleratedSoakKeepsReplayAndWorkerResourcesBounded |
| `store.go` | workflow | Source API and private invariants |
| `store_test.go` | workflow_test | Existing source evidence: TestHistoryPageProvidesStableBoundedPagination, TestStoreCommitErrorMakesDurabilityExplicit, TestHistoryQueryRejectsInvalidOrUnboundedCursors, TestHistoryPageRejectsUnstableAdapterOutput |
| `transition.go` | workflow | Source API and private invariants |
| `transition_fuzz_test.go` | workflow_test | Existing source evidence: FuzzPendingWorkValidation, FuzzTransitionValidation |
| `transition_test.go` | workflow_test | Existing source evidence: TestTransitionOwnsAtomicHistoryAndDueWork, TestTransitionRejectsNonAtomicOrUnboundedPlans, TestPendingWorkRejectsInvalidOrUnboundedMetadata, TestPendingWorkAcceptsEveryKindAndExactPayloadLimit, TestTransitionValidatesCreateAndOrderedEventBatch, TestTransitionEnforcesBatchAndWorkAdmissionBounds, TestTransitionBoundsAggregatePersistedPayload, TestTransitionBoundsAggregatePayloadAcrossEvents |
| `version_skew_test.go` | workflow_test | Existing source evidence: TestMixedVersionRollingDeployKeepsWorkersPinnedToDurableDefinitions |
| `wait_fuzz_test.go` | workflow_test | Existing source evidence: FuzzSignalAcceptanceBoundaries |
| `wait_history.go` | workflow | Source API and private invariants |
| `wait_history_internal_test.go` | workflow | Existing source evidence: TestApplyWaitRejectsNonWaitEvents, TestWaitExtensionPreservesLegacySnapshotDigestsWhenNoWaitsExist, TestWaitSnapshotsPreserveNilAndPersistedProgress |
| `wait_history_test.go` | workflow_test | Existing source evidence: TestReplayReconstructsDurableTimersAndSignals, TestHistoryEventsRejectInvalidTimerAndSignalFields, TestReplayRejectsInvalidTimerAndSignalTransitions |
| `wait_transition.go` | workflow | Source API and private invariants |
| `wait_transition_test.go` | workflow_test | Existing source evidence: TestTimerScheduleCreatesAtomicDueWorkAndHistory, TestSignalAcceptanceCreatesAnIdempotentDurableTransition, TestTimerFireCreatesThePersistedDecisionBeforeCompletion, TestTimerAndSignalBuildersRejectWrongStepsAndBounds |
| `work.go` | workflow | Source API and private invariants |
| `work_fuzz_test.go` | workflow_test | Existing source evidence: FuzzWorkLeaseBoundaries |
| `work_test.go` | workflow_test | Existing source evidence: TestWorkClaimRequestIsBoundedAndImmutable, TestWorkLeaseCarriesAnImmutableFencingToken, TestWorkLeaseCommandsRequireTheCurrentFence, TestWorkLeaseCannotOutliveTheWorkDeadline |
| `worker.go` | workflow | Source API and private invariants |
| `worker_fuzz_test.go` | workflow_test | Existing source evidence: FuzzWorkDecisionBoundaries |
| `worker_internal_test.go` | workflow | Existing source evidence: TestWorkDecisionCompleteValidityRequiresEmptyMetadata, TestFairLeaseOrderRoundRobinsTenantsWithoutReorderingWithinTenant |
| `worker_test.go` | workflow_test | Existing source evidence: TestWorkDecisionMakesTerminalHandlingExplicit, TestWorkerRunsWithBoundedConcurrencyAndCompletesPersistedDecisions, TestWorkerImmediatelyRefillsAvailableCapacity, TestWorkerDoesNotClaimPastCapacityWhileAProcessorIsActive, TestWorkerPollsAfterAnEmptyClaim, TestWorkerRenewsLeaseAndCancelsAStaleProcessor, TestNewWorkerRejectsUnboundedOrMissingConfiguration, TestWorkerRunValidatesRuntimeBoundariesAndStopsIdlePolling, TestWorkerHandleAppliesRetryAndDeadLetterDecisions, TestWorkerHandleRenewsAndFinalizesWithTheCurrentLease, TestWorkerHandleLeavesFailedOrInvalidProcessingForRecovery, TestWorkerHandleCancellationPreservesOnlyKnownDecisions, TestWorkerHandleRejectsInvalidRenewalAndFinalizationTimes, TestWorkerHooksObserveClaimProcessingAndLeaseLoss |

## Investigation and verification disposition

Inventory preceded source edits. All 29 production files were traced across
construction, replay, processor/worker dispatch and PostgreSQL persistence.
The 74 assurance files retain their assertion/helper/fixture/entrypoint roles;
the table identifies each Test/Fuzz/Benchmark/Example and integration tag.
Selected existing assertion bodies were inspected as source evidence, not
reported as newly executed tests. No test text or expected outcome is rewritten.

Investigated contract decisions and direct evidence:

- Definitions own steps/branches/compensations, but copy before validation;
  registries borrow migration callbacks and do not execute them. Digests bind
  the actual owned Go JSON representation, not external canonical JSON or
  authentication. Definition, admission-copy, version-skew and replay tests
  retain different accepted-value, allocation-order and deployment oracles.
- History validates event-specific fields, replay enforces exact pins,
  contiguous sequences, monotonic persisted time, lifecycle and step policies.
  UTC normalization removes monotonic clock readings. Replay never invokes a
  migration callback; migrated bytes are already persisted. Existing model,
  history and wait snapshot tests remain the behavioral authorities.
- Activity, child and compensation processors commit admission before calling
  borrowed code; running redelivery becomes unknown instead of redispatch.
  Known retryable failure uses capped deterministic backoff and remaining
  attempt/deadline policy. Unknown state and manual resolution are not success.
  Processor tests assert both sides of the admission boundary, panic/unknown
  classification and exact reconciliation. Wrappers do not contain store/clock
  panics or forcibly stop blocking callbacks.
- Compensation is explicitly admitted for a successful activity; inspection
  uses persisted scheduling order. There is no automatic reverse-order
  rollback or compensation-on-failure supervisor. The compensation recipe,
  history and operator tests retain this distinction; current architecture
  wording now identifies application-owned order.
- Orchestration uses ordered persisted progress. Parallel admission must fit
  transition batch limits; unknown outcomes wait. Race candidates are compared
  by persisted received time, with declared branch order breaking ties, then
  replay follows the recorded winner. Existing parallel/race/child tests are
  unchanged; this is not an arbitrary graph engine or live arrival oracle.
- Worker Run bounds active handlers, not all goroutines or direct Handle calls.
  It stops admission and joins cooperating processors, ordinarily returning nil
  on cancellation. Handle may finalize a known decision after cancellation using
  retained context values and detached cancellation. FinalizeTimeout bounds
  cooperative store context, not processor joins, hooks or total shutdown.
  Existing worker cancellation, renewal, hook and hardening assertions were
  inspected; noncooperation and panic containment are caller boundaries.
- PostgreSQL Commit stages history, identity and work in one transaction;
  Stage never commits or rolls back the caller transaction. Receipt uncertainty
  is not known absence. Work mutations use supplied caller time and compare
  state/owner/token/expiry; Renew can shorten expiry. Claim query/scan/receipt
  failures can leave unseen leases. Transition Commit does not query a work
  lease fence: plan validation is not external-effect fencing. Existing store
  boundary and integration source retains transaction/deadlock/recovery proof.
- PostgreSQL signed bigint and integer storage are narrower than generic Go
  uint64/uint32 carriers. DDL is caller-applied; Down is destructive. Pagination
  cursors provide stable keys, not cross-page snapshots, authorization or
  retention. The adapter owns rows/rollback cleanup, not the borrowed pool.
- Opaque payloads, raw error causes, callback state, audit identities and hooks
  have no automatic redaction/encryption. Current reference wording no longer
  calls underlying causes safe. Store wrappers suppress diagnostic text while
  exposing originals through Unwrap; tenant metadata does not grant isolation.
- Dispatch/operator decoding bounds bytes and rejects unknown fields/trailing
  values through encoding/json, but preserves its duplicate-key and
  case-insensitive matching semantics. Bounds apply to accepted values/counts,
  not whole-process heap, intake, retention or callback work.

Specification/provenance: modules.json declares no specification, conformance
corpus or source pin, and there is no typed decision-history directory to
rewrite. The authoritative v3 module/pgx 5.11 seam and historical v1/v2 source
models remain byte-preserved. No normative fixture is changed or standards,
exactly-once, backend qualification or process-wide performance claim is added.

Assurance/resource boundaries: 15 fuzz entrypoints and two benchmarks are
source-inspected, not run. Two existing root Examples and README quickstart are
byte-unchanged and executed through the configured docs operation below; the
in-memory recipe is not durable backend qualification. Seven
integration-tag files can start PostgreSQL18 Docker fixtures, process-death
subprocesses, restart/snapshot, failover and deadlock exercises; they are not
executed for prose. Test helpers own fakes and manual clocks, and integration
fixtures retain their original explicit cleanup. No TestMain or generated
producer was found.

Remaining maintainer questions, not missing documentation or authorized fixes:
external outcome reconciliation, compensation dependency order, trusted clock
policy and practical deadlines, callback cooperation, tenant authorization,
retention/encryption/redaction and production backup/recovery remain application
or store-implementation decisions. No runtime defect is repaired in this pass.
Bounded native verification used the exclusive coordinator grant and Go1.27.2.
All three exact owned caches, helper, renders, quickstart copies and newly
created gate output were removed after the session, including failure handling.

## Bounded verification and remaining limits

- Pinned Go1.27.2 gofmt covered all 27 changed production files. A stdlib parser,
  lexer and go/doc/comment proof covered all 103 tracked Go files: executable
  AST/signatures and raw noncomment tokens/literals were identical, with no
  raw formatter-semicolon deltas; all directive/marker lines were exact.
  It checked 1080 name-led public declarations/fields/interface methods,
  both canonical package comments and 1114 parsed comment HTML groups.
- Native go doc -all . and go doc -all ./postgres rendered the full contracts
  (139621 and 7328 bytes). Required ownership/lifecycle/parser/failure-window
  wording appeared in the rendered output. No prose snapshot is configured.
- make -f verification/package.mk docs ran the actual repository operation
  once: README quickstart go run, both package Go docs and exactly the two
  existing root Examples selected with native jq pass assertions. It passed.
  The Examples and expected outputs were not modified. No separate duplicate
  Example run, integration-tag test, Docker or PostgreSQL service was started.
- The coordinator's existing development CLI passed source-docs check --json,
  docs check --local and offline specification check (0 configured modules,
  0 decisions). These are structure/link/current-record checks, not semantic,
  cryptographic, backend, CI or public-release qualification.
- Binary apidiff api/v3-baseline.txt, Go module/dependency/catalog/tool pins,
  historical v1/v2/v3 migration/threat records, all assurance bodies, public
  examples, scripts, fixture bytes and every untouched tracked file remain
  byte-preserved. No normative expectation, generated data or API producer
  output was rewritten.
- Affected Markdown relative links (3), all-source markers and whitespace passed
  local inspection. The coordinator's pinned default secret scanner found zero
  findings in complete owned content (415668 bytes) and changed/added content
  (117894 bytes), with no allowlist bypass. Staged content is checked again
  immediately before commit. Ordinary complete final-diff inspection covered
  usefulness, truth, scope and unchanged runtime; this is Tier A, not an
  independently attested behavioral or release review.

No broad suite, vet/race/leak/fuzz/benchmark/soak/conformance, live service,
interoperability/public consumer, online authority fetch or release campaign
was run. Existing tests and hosted/release artifacts are retained source
references, not newly passed qualification. This worker does not publish or
qualify remote main; the coordinator owns delivery and exact-source CI.

## Objective Enforcement

The checksum-pinned public Tools v2.4.0 verifier now supplies
`make source-docs-check` as a prerequisite of the existing local and CI Make
routes. Ordinary shared CI selects strict source-documentation checking and
requires its result. Native prerequisites, operation budgets, security and
release controls are retained. Hosted exact-head evidence is recorded by the
coordination ledger; configuration alone does not establish green delivery.
