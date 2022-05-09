package mgmt

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/golang/mock/gomock"
	awsprovider "github.com/openshift/osdctl/pkg/provider/aws"
	"github.com/openshift/osdctl/pkg/provider/aws/mock"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestIsOwned(t *testing.T) {
	var genericAWSError error = fmt.Errorf("Generic AWS error")
	testData := []struct {
		testname         string
		tags             organizations.ListTagsForResourceOutput
		expectedIsOwned  bool
		expectErr        error
		expectedAWSError error
	}{
		{
			testname: "test for unowned account",
			tags: organizations.ListTagsForResourceOutput{
				Tags: []*organizations.Tag{},
			},
			expectedIsOwned:  false,
			expectErr:        nil,
			expectedAWSError: nil,
		},
		{
			testname: "test for owned account",
			tags: organizations.ListTagsForResourceOutput{
				Tags: []*organizations.Tag{
					{
						Key:   aws.String("claimed"),
						Value: aws.String("true"),
					},
				},
			},
			expectedIsOwned:  true,
			expectErr:        nil,
			expectedAWSError: nil,
		},
		{
			testname: "test for owned account, encounter aws error",
			tags: organizations.ListTagsForResourceOutput{
				Tags: []*organizations.Tag{
					{
						Key:   aws.String("claimed"),
						Value: aws.String("true"),
					},
				},
			},
			expectedIsOwned:  false,
			expectErr:        genericAWSError,
			expectedAWSError: genericAWSError,
		},
	}
	for _, test := range testData {
		t.Run(test.testname, func(t *testing.T) {
			mocks := setupDefaultMocks(t, []runtime.Object{})
			mockAWSClient := mock.NewMockClient(mocks.mockCtrl)
			accountID := "11111"

			mockAWSClient.EXPECT().ListTagsForResource(
				&organizations.ListTagsForResourceInput{
					ResourceId: aws.String(accountID),
				},
			).Return(&test.tags, test.expectedAWSError)

			var awsC awsprovider.Client = mockAWSClient
			isOwned, err := isOwned(accountID, &awsC)

			if isOwned != test.expectedIsOwned {
				t.Errorf("expected isOwned to be %v, got %v", test.expectedIsOwned, isOwned)
			}

			if err != test.expectErr {
				t.Errorf("expected error to be %v, got %v", test.expectErr, err)
			}
		})
	}
}

func TestFindUntaggedAccount(t *testing.T) {
	var genericAWSError error = fmt.Errorf("Generic AWS error")

	testData := []struct {
		name              string
		accountsList      []string
		tags              map[string]string
		suspendCheck      bool
		accountStatus     string
		expectedAccountId string
		expectErr         error
		expectedAWSError  error
	}{
		{
			name:              "test for untagged account present",
			accountsList:      []string{"111111111111"},
			expectedAccountId: "111111111111",
			tags:              map[string]string{},
			suspendCheck:      true,
			accountStatus:     organizations.AccountStatusActive,
			expectErr:         nil,
			expectedAWSError:  nil,
		},
		{
			name:              "test for only partially tagged accounts present",
			accountsList:      []string{"111111111111"},
			expectedAccountId: "",
			tags: map[string]string{
				"claimed": "true",
			},
			suspendCheck:     false,
			expectErr:        ErrNoUntaggedAccounts,
			expectedAWSError: nil,
		},
		{
			name:              "test for only tagged accounts present",
			accountsList:      []string{"111111111111"},
			expectedAccountId: "",
			tags: map[string]string{
				"owner":   "randuser",
				"claimed": "true",
			},
			suspendCheck:     false,
			expectErr:        ErrNoUntaggedAccounts,
			expectedAWSError: nil,
		},
		{
			name:              "test for AWS list accounts error",
			accountsList:      []string{},
			expectedAccountId: "",
			tags:              nil,
			suspendCheck:      false,
			expectErr:         genericAWSError,
			expectedAWSError:  genericAWSError,
		},
		{
			name:              "test for suspended account error",
			accountsList:      []string{"111111111111"},
			expectedAccountId: "",
			tags:              map[string]string{},
			suspendCheck:      true,
			accountStatus:     organizations.AccountStatusSuspended,
			expectErr:         ErrNoUntaggedAccounts,
			expectedAWSError:  nil,
		},
	}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {

			mocks := setupDefaultMocks(t, []runtime.Object{})

			mockAWSClient := mock.NewMockClient(mocks.mockCtrl)
			rootOuId := "abc"
			o := &accountAssignOptions{}
			o.awsClient = mockAWSClient

			awsOutputAccounts := &organizations.ListAccountsForParentOutput{}

			if test.accountsList != nil {
				accountsList := []*organizations.Account{}
				for _, a := range test.accountsList {
					account := &organizations.Account{
						Id: aws.String(a),
					}
					accountsList = append(accountsList, account)
				}
				awsOutputAccounts.Accounts = accountsList
			}

			if test.tags != nil {
				awsOutputTags := &organizations.ListTagsForResourceOutput{}
				tags := []*organizations.Tag{}
				for key, value := range test.tags {
					tag := &organizations.Tag{
						Key:   aws.String(key),
						Value: aws.String(value),
					}
					tags = append(tags, tag)
				}
				awsOutputTags.Tags = tags

				mockAWSClient.EXPECT().ListTagsForResource(
					&organizations.ListTagsForResourceInput{
						ResourceId: &test.accountsList[0],
					}).Return(
					awsOutputTags,
					test.expectedAWSError,
				)
			}

			if test.suspendCheck {
				mockAWSClient.EXPECT().DescribeAccount(
					&organizations.DescribeAccountInput{
						AccountId: &test.accountsList[0],
					},
				).Return(
					&organizations.DescribeAccountOutput{
						Account: &organizations.Account{
							Id:     aws.String(test.accountsList[0]),
							Status: aws.String(test.accountStatus),
						},
					}, nil,
				)
			}

			mockAWSClient.EXPECT().ListAccountsForParent(gomock.Any()).Return(
				awsOutputAccounts,
				test.expectedAWSError,
			)

			returnValue, err := o.findUntaggedAccount(rootOuId)
			if test.expectErr != err {
				t.Errorf("expected error %s and got %s", test.expectErr, err)
			}
			if returnValue != test.expectedAccountId {
				t.Errorf("expected %s is %s", test.expectedAccountId, returnValue)
			}
		})
	}
}

func TestCreateAccount(t *testing.T) {
	mocks := setupDefaultMocks(t, []runtime.Object{})

	mockAWSClient := mock.NewMockClient(mocks.mockCtrl)

	seed := int64(1)
	rand.Seed(seed)
	randStr := RandomString(6)
	accountName := "osd-creds-mgmt+" + randStr
	email := accountName + "@redhat.com"

	createId := "car-random1234"

	mockAWSClient.EXPECT().CreateAccount(&organizations.CreateAccountInput{
		AccountName: &accountName,
		Email:       &email,
	}).Return(&organizations.CreateAccountOutput{
		CreateAccountStatus: &organizations.CreateAccountStatus{Id: &createId},
	}, nil)

	expectedOutput := "SUCCEEDED"

	awsDescribeOutput := &organizations.DescribeCreateAccountStatusOutput{
		CreateAccountStatus: &organizations.CreateAccountStatus{
			State: &expectedOutput,
		}}

	mockAWSClient.EXPECT().DescribeCreateAccountStatus(&organizations.DescribeCreateAccountStatusInput{
		CreateAccountRequestId: &createId,
	}).Return(awsDescribeOutput, nil)

	o := &accountAssignOptions{}
	o.awsClient = mockAWSClient
	returnVal, err := o.createAccount(seed)
	if err != nil {
		t.Error("failed to create account")
	}
	if returnVal.CreateAccountStatus.State != &expectedOutput {
		t.Error("failed to create account")
	}
}

func TestTagAccount(t *testing.T) {

	mocks := setupDefaultMocks(t, []runtime.Object{})

	mockAWSClient := mock.NewMockClient(mocks.mockCtrl)
	accountID := "111111111111"

	awsOutputTag := &organizations.TagResourceOutput{}

	mockAWSClient.EXPECT().TagResource(gomock.Any()).Return(
		awsOutputTag,
		nil,
	)

	o := &accountAssignOptions{}
	o.awsClient = mockAWSClient
	err := o.tagAccount(accountID)
	if err != nil {
		t.Errorf("failed to tag account")
	}
}

func TestMoveAccount(t *testing.T) {

	mocks := setupDefaultMocks(t, []runtime.Object{})

	mockAWSClient := mock.NewMockClient(mocks.mockCtrl)

	accountId := "111111111111"
	destOu := "abc-vnjfdshs"
	rootOu := "abc"

	awsOutputMove := &organizations.MoveAccountOutput{}

	mockAWSClient.EXPECT().MoveAccount(gomock.Any()).Return(
		awsOutputMove,
		nil,
	)

	o := &accountAssignOptions{}
	o.awsClient = mockAWSClient
	err := o.moveAccount(accountId, destOu, rootOu)
	if err != nil {
		t.Errorf("failed to move account")
	}
}
