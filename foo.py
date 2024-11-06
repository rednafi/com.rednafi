from typing import override


class Animal:
    @property
    def species(self) -> str:
        return "Unknown"


class Cat(Animal):
    @override
    @property
    def species(self) -> str:
        return "Catus"
