// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

// Binder errors are in the [500-600) range.
var (
	ErrorInvalidPackageName       = newError(500, "The package name must be a valid identifier")
	ErrorMalformedPackageURL      = newError(501, "Package URL '%v' is malformed: %v")
	ErrorImportNotFound           = newError(502, "The imported package '%v' was not found; has it been installed?")
	ErrorTypeNotFound             = newError(503, "Type '%v' could not be found: %v")
	ErrorSymbolNotFound           = newError(504, "Symbol '%v' could not be found: %v")
	ErrorSymbolAlreadyExists      = newError(505, "A symbol already exists with the name '%v'")
	ErrorIncorrectExprType        = newError(506, "Expression has an incorrect type; expected '%v', got '%v'")
	ErrorMemberNotAccessible      = newError(507, "Member '%v' is not accessible (it is %v)")
	ErrorExpectedReturnExpr       = newError(508, "Expected a return expression of type %v")
	ErrorUnexpectedReturnExpr     = newError(509, "Unexpected return expression; function has no return type (void)")
	ErrorDuplicateLabel           = newError(510, "Duplicate label '%v': %v")
	ErrorUnknownJumpLabel         = newError(511, "Unknown label '%v' used in the %v statement")
	ErrorCannotNewAbstractClass   = newError(512, "Cannot `new` an abstract class '%v'")
	ErrorIllegalObjectLiteralType = newError(513,
		"The type '%v' may not be used as an object literal type; only records and interfaces are permitted")
	ErrorMissingRequiredProperty        = newError(514, "Missing required property '%v'")
	ErrorUnaryOperatorInvalidForType    = newError(515, "The operator %v is invalid on operand type %v; expected %v")
	ErrorUnaryOperatorInvalidForOperand = newError(516, "The operator %v is invalid on operand %v; expected %v")
	ErrorUnaryOperatorMustBePrefix      = newError(517, "The operator %v must be in a prefix position (not postfix)")
	ErrorBinaryOperatorInvalidForType   = newError(518, "The operator %v is invalid on %v operand type %v; expected %v")
	ErrorIllegalAssignmentLValue        = newError(519, "Cannot assign to the target LHS expression")
	ErrorIllegalNumericAssignmentLValue = newError(520, "Cannot perform numeric assignment %v on a non-numeric LHS")
	ErrorIllegalAssignmentTypes         = newError(521, "Cannot assign a value of type %v to target of type %v")
)