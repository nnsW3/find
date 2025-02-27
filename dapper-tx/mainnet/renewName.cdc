import DapperUtilityCoin from 0xead892083b3e2c6c
import FIND from 0x097bafa4e0b48eef

transaction(merchAccount: Address, name: String, amount: UFix64) {

    let price : UFix64
    let finLeases : &FIND.LeaseCollection
    let mainDapperUtilityCoinVault: &DapperUtilityCoin.Vault
    let balanceBeforeTransfer: UFix64

    prepare(dapper: AuthAccount, acct: AuthAccount) {
        self.price=FIND.calculateCost(name)
        self.mainDapperUtilityCoinVault = dapper.borrow<&DapperUtilityCoin.Vault>(from: /storage/dapperUtilityCoinVault) ?? panic("Cannot borrow DapperUtilityCoin vault from account storage".concat(dapper.address.toString()))
        self.balanceBeforeTransfer = self.mainDapperUtilityCoinVault.balance
        self.finLeases= acct.borrow<&FIND.LeaseCollection>(from:FIND.LeaseStoragePath) ?? panic("Could not borrow reference to find lease collection")
    }

    pre{
        merchAccount == 0x55459409d30274ee : "Merchant account is not .find"
        self.price == amount : "expected renew cost : ".concat(self.price.toString()).concat(" is not the same as calculated renew cost : ").concat(amount.toString())
    }

    execute{
        let payVault <- self.mainDapperUtilityCoinVault.withdraw(amount: self.price) as! @DapperUtilityCoin.Vault
        let finToken= self.finLeases.borrow(name)
        finToken.extendLeaseDapper(merchAccount: merchAccount, vault: <- payVault)
    }

    post {
        self.mainDapperUtilityCoinVault.balance == self.balanceBeforeTransfer: "DapperUtilityCoin leakage"
    }
}
